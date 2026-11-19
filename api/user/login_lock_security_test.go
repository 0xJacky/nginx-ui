package user

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// size reports how many client entries are held or waited for.
func (l *clientLocks) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.locks)
}

// stubLoginFailureDelay makes every failed password login wait d for the rest
// of the test. The returned channel receives a value each time a failed login
// starts its delay.
func stubLoginFailureDelay(t *testing.T, d time.Duration) <-chan struct{} {
	t.Helper()
	entered := make(chan struct{}, 128)
	original := loginFailureDelay
	loginFailureDelay = func() time.Duration {
		select {
		case entered <- struct{}{}:
		default:
		}
		return d
	}
	t.Cleanup(func() { loginFailureDelay = original })
	return entered
}

// startFailedLoginInDelay sends a wrong password from ip and returns once that
// request is inside its failure delay. The test waits for the request to
// finish before it ends.
func startFailedLoginInDelay(t *testing.T, router http.Handler, ip string, entered <-chan struct{}) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		postLogin(router, ip, "", "wrong")
	}()
	t.Cleanup(func() { <-done })
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the failed login never reached its failure delay")
	}
}

// useFileDB moves the fixture onto a SQLite file, which is what Nginx UI runs
// on. The shared in-memory database of the fixture refuses concurrent writers
// with "database table is locked" instead of waiting, so tests that send
// logins from many clients at once need the file to behave like production.
func useFileDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "login.db")), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}, &model.BanIP{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)
	hash, err := bcrypt.GenerateFromPassword([]byte(verifyPassword), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.User{
		Model: model.Model{ID: 1}, Name: "admin", Password: string(hash), Status: true, Language: "en",
	}).Error)
	return db
}

// passwordIncorrectCode is the response code of a wrong password.
func passwordIncorrectCode(t *testing.T) int {
	t.Helper()
	var e *cosy.Error
	require.True(t, errors.As(internaluser.ErrPasswordIncorrect, &e))
	return int(e.Code)
}

func requireNoClientLocksLeft(t *testing.T) {
	t.Helper()
	require.Zero(t, loginClientLocks.size(), "login client locks must be dropped once no request uses them")
}

// Attack side: a wrong password from one client must not delay the correct
// login of another client while the failed request is still in its delay.
func TestLoginFailureDelayDoesNotBlockOtherClients(t *testing.T) {
	router, _ := setupLoginBanFixture(t, nil)
	const delay = 2 * time.Second
	entered := stubLoginFailureDelay(t, delay)

	startFailedLoginInDelay(t, router, verifyAttackerIP, entered)

	start := time.Now()
	status, resp := postLogin(router, verifyOtherIP, "", verifyPassword)
	elapsed := time.Since(start)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, LoginSuccess, resp.Code)
	require.Less(t, elapsed, delay/2,
		"a correct login from another client waited for a failed login's delay")
	// The failed login is still in its delay, and it holds no lock there.
	requireNoClientLocksLeft(t)
}

// Attack side: a banned client is refused at once, without waiting for logins
// of other clients that are in flight.
func TestBannedClientIsRefusedWithoutWaitingForOtherClients(t *testing.T) {
	router, _ := setupLoginBanFixture(t, nil)
	const delay = 2 * time.Second
	entered := stubLoginFailureDelay(t, delay)
	for range settings.AuthSettings.MaxAttempts {
		internaluser.BanIP(verifyOtherIP)
	}

	startFailedLoginInDelay(t, router, verifyAttackerIP, entered)

	start := time.Now()
	status, resp := postLogin(router, verifyOtherIP, "", verifyPassword)
	elapsed := time.Since(start)
	require.Equal(t, http.StatusTooManyRequests, status)
	require.Equal(t, ErrMaxAttempts, resp.Code)
	require.Less(t, elapsed, delay/2, "a banned client waited for another client's login")
}

// Attack side: a banned client is refused before it takes its own login
// lock, so a pile of requests from it does not queue behind one another.
func TestBannedClientIsRefusedWhileItsLockIsHeld(t *testing.T) {
	router, _ := setupLoginBanFixture(t, nil)
	for range settings.AuthSettings.MaxAttempts {
		internaluser.BanIP(verifyAttackerIP)
	}

	unlock := loginClientLocks.lock(verifyAttackerIP)
	type result struct {
		status int
		resp   LoginResponse
	}
	answered := make(chan result, 1)
	go func() {
		status, resp := postLogin(router, verifyAttackerIP, "", verifyPassword)
		answered <- result{status, resp}
	}()

	var got result
	select {
	case got = <-answered:
		unlock()
	case <-time.After(time.Second):
		unlock()
		<-answered
		t.Fatal("a banned client waited for its login lock before being refused")
	}
	require.Equal(t, http.StatusTooManyRequests, got.status)
	require.Equal(t, ErrMaxAttempts, got.resp.Code)
	requireNoClientLocksLeft(t)
}

// One client cannot get more than MaxAttempts guesses evaluated by sending
// them concurrently: every request must see the failures recorded before it.
// Wrong OTPs after the correct password take the failure path without the
// failure delay.
func TestLoginPerClientLimitHoldsUnderConcurrency(t *testing.T) {
	router, _ := setupLoginBanFixture(t, nil)
	db := useFileDB(t)
	secret := enableOTP(t, db)

	var wg sync.WaitGroup
	var evaluated atomic.Int64
	for i := range 5 * settings.AuthSettings.MaxAttempts {
		code := wrongOTP(t, secret, i)
		wg.Go(func() {
			if status, _ := postLoginOTP(router, verifyAttackerIP, code); status != http.StatusTooManyRequests {
				evaluated.Add(1)
			}
		})
	}
	wg.Wait()
	require.LessOrEqual(t, evaluated.Load(), int64(settings.AuthSettings.MaxAttempts),
		"concurrent guesses from one client slipped past the per-client limit")
	requireNoClientLocksLeft(t)
}

// Attack side: the same limit holds for concurrent wrong passwords, and the
// failure delay being outside the lock does not open a gap between the check
// and the recorded failure.
func TestLoginPerClientPasswordLimitHoldsUnderConcurrency(t *testing.T) {
	router, _ := setupLoginBanFixture(t, nil)
	db := useFileDB(t)
	stubLoginFailureDelay(t, 0)

	var wg sync.WaitGroup
	var evaluated, refused atomic.Int64
	total := 5 * settings.AuthSettings.MaxAttempts
	for i := range total {
		wg.Go(func() {
			status, resp := postLogin(router, verifyAttackerIP, "", fmt.Sprintf("wrong-%d", i))
			if status == http.StatusTooManyRequests && resp.Code == ErrMaxAttempts {
				refused.Add(1)
				return
			}
			evaluated.Add(1)
		})
	}
	wg.Wait()
	require.EqualValues(t, settings.AuthSettings.MaxAttempts, evaluated.Load(),
		"concurrent passwords from one client slipped past the per-client limit")
	require.EqualValues(t, total-settings.AuthSettings.MaxAttempts, refused.Load())

	rows := banRows(t, db, verifyAttackerIP)
	require.Len(t, rows, 1)
	require.Equal(t, settings.AuthSettings.MaxAttempts, rows[0].Attempts)
	requireNoClientLocksLeft(t)
}

// The failure delay still applies to the client that failed.
func TestLoginFailureDelayStillSlowsTheFailingClient(t *testing.T) {
	router, db := setupLoginBanFixture(t, nil)
	const delay = 300 * time.Millisecond
	stubLoginFailureDelay(t, delay)

	start := time.Now()
	status, resp := postLogin(router, verifyAttackerIP, "", "wrong")
	require.NotEqual(t, http.StatusOK, status)
	require.Equal(t, passwordIncorrectCode(t), resp.Code)
	require.GreaterOrEqual(t, time.Since(start), delay)
	require.Len(t, banRows(t, db, verifyAttackerIP), 1)
	requireNoClientLocksLeft(t)
}

// Legitimate side: wrong passwords below the limit do not block the next
// correct login, and the success clears the client's ban record.
func TestLoginSucceedsAfterWrongPasswordsBelowLimit(t *testing.T) {
	router, db := setupLoginBanFixture(t, nil)
	stubLoginFailureDelay(t, 0)

	for i := range settings.AuthSettings.MaxAttempts - 1 {
		status, resp := postLogin(router, verifyAttackerIP, "", fmt.Sprintf("wrong-%d", i))
		require.NotEqual(t, http.StatusTooManyRequests, status)
		require.Equal(t, passwordIncorrectCode(t), resp.Code)
	}
	require.Len(t, banRows(t, db, verifyAttackerIP), 1)

	status, resp := postLogin(router, verifyAttackerIP, "", verifyPassword)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, LoginSuccess, resp.Code)
	require.NotNil(t, resp.AccessTokenPayload)
	require.NotEmpty(t, resp.Token)
	require.Empty(t, banRows(t, db, verifyAttackerIP), "a successful login must clear the ban record")
	requireNoClientLocksLeft(t)
}

// Legitimate side: one client can log in several times in a row, and many
// clients can log in at the same time.
func TestLoginRepeatedAndConcurrentCorrectLogins(t *testing.T) {
	router, _ := setupLoginBanFixture(t, nil)
	useFileDB(t)

	for range 3 {
		status, resp := postLogin(router, verifyOtherIP, "", verifyPassword)
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, LoginSuccess, resp.Code)
	}

	var wg sync.WaitGroup
	var succeeded atomic.Int64
	const clients = 20
	for i := range clients {
		ip := fmt.Sprintf("192.0.2.%d", 100+i)
		for range 2 {
			wg.Go(func() {
				status, resp := postLogin(router, ip, "", verifyPassword)
				if status == http.StatusOK && resp.Code == LoginSuccess {
					succeeded.Add(1)
				}
			})
		}
	}
	wg.Wait()
	require.EqualValues(t, 2*clients, succeeded.Load())
	requireNoClientLocksLeft(t)
}

// floodFailedLogins sends wrong passwords from ever new client IPs until the
// returned stop function is called, and reports how many it sent.
func floodFailedLogins(router http.Handler) (stop func() int64) {
	done := make(chan struct{})
	var wg sync.WaitGroup
	var sent atomic.Int64
	for range 16 {
		wg.Go(func() {
			for {
				select {
				case <-done:
					return
				default:
				}
				n := sent.Add(1)
				postLogin(router, fmt.Sprintf("10.%d.%d.%d", n>>16&255, n>>8&255, n&255), "", "wrong")
			}
		})
	}
	return func() int64 {
		close(done)
		wg.Wait()
		return sent.Load()
	}
}

// Legitimate side: a correct recovery code logs in while other clients fail
// logins at the same time. Consuming the code is a database write, so it must
// not collide with the attempts the failed logins record, and a collision
// must not count against the client as a wrong code.
func TestRecoveryCodeLoginSucceedsDuringFailedLoginsOfOtherClients(t *testing.T) {
	router, _ := setupLoginBanFixture(t, nil)
	db := useFileDB(t)
	enableOTP(t, db)
	stubLoginFailureDelay(t, 0)

	codes := make([]*model.RecoveryCode, 400)
	for i := range codes {
		codes[i] = &model.RecoveryCode{Code: fmt.Sprintf("%05d-%05d", i, i)}
	}
	require.NoError(t, db.Model(&model.User{Model: model.Model{ID: 1}}).Select("recovery_codes").
		Updates(&model.User{RecoveryCodes: model.RecoveryCodes{Codes: codes}}).Error)

	stop := floodFailedLogins(router)
	var failures []string
	deadline := time.Now().Add(2 * time.Second)
	logins := 0
	for _, code := range codes {
		if !time.Now().Before(deadline) {
			break
		}
		status, resp := postLoginRequest(router, verifyOtherIP, "",
			LoginUser{Name: "admin", Password: verifyPassword, RecoveryCode: code.Code})
		logins++
		if status != http.StatusOK || resp.Code != LoginSuccess {
			failures = append(failures, fmt.Sprintf("%d %d %s", status, resp.Code, resp.Message))
		}
	}
	sent := stop()
	t.Logf("%d recovery code logins alongside %d failed logins", logins, sent)
	require.Empty(t, failures, "recovery code logins failed while other clients were failing")
	require.Empty(t, banRows(t, db, verifyOtherIP))
	requireNoClientLocksLeft(t)
}

// Legitimate side: failed logins from many clients at once do not make a
// correct login of another client fail. Failed logins are no longer
// serialized with each other, so the database writes they make must not
// collide with the one that issues the login token.
func TestCorrectLoginSucceedsDuringFailedLoginsOfOtherClients(t *testing.T) {
	router, _ := setupLoginBanFixture(t, nil)
	useFileDB(t)
	stubLoginFailureDelay(t, 0)

	stop := floodFailedLogins(router)
	var failures []string
	deadline := time.Now().Add(2 * time.Second)
	logins := 0
	for time.Now().Before(deadline) {
		status, resp := postLogin(router, verifyOtherIP, "", verifyPassword)
		logins++
		if status != http.StatusOK || resp.Code != LoginSuccess {
			failures = append(failures, fmt.Sprintf("%d %s", status, resp.Message))
		}
	}
	sent := stop()
	t.Logf("%d correct logins alongside %d failed logins", logins, sent)
	require.Empty(t, failures, "correct logins failed while other clients were failing")
	requireNoClientLocksLeft(t)
}

// Legitimate side: the password-then-OTP flow still works.
func TestLoginTwoFactorFlowStillWorks(t *testing.T) {
	router, db := setupLoginBanFixture(t, nil)
	secret := enableOTP(t, db)

	status, resp := postLogin(router, verifyOtherIP, "", verifyPassword)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, Enabled2FA, resp.Code)
	require.Nil(t, resp.AccessTokenPayload)

	status, _ = postLoginOTP(router, verifyOtherIP, wrongOTP(t, secret, 0))
	require.NotEqual(t, http.StatusOK, status)
	require.NotEqual(t, http.StatusTooManyRequests, status)

	status, resp = postLoginOTP(router, verifyOtherIP, currentOTP(t, secret))
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, LoginSuccess, resp.Code)
	require.NotEmpty(t, resp.Token)
	require.NotEmpty(t, resp.SecureSessionID)
	require.Empty(t, banRows(t, db, verifyOtherIP))
	requireNoClientLocksLeft(t)
}

// The lock serializes one key, leaves other keys free, tolerates a second
// release and drops its entry once nobody holds or waits for it.
func TestClientLocks(t *testing.T) {
	locks := &clientLocks{locks: map[string]*clientLock{}}

	unlockA := locks.lock("a")
	unlockB := locks.lock("b") // a different key does not wait for "a"
	require.Equal(t, 2, locks.size())

	acquired := make(chan func())
	go func() { acquired <- locks.lock("a") }()
	select {
	case <-acquired:
		t.Fatal("a second holder got the lock for the same key")
	case <-time.After(50 * time.Millisecond):
	}

	unlockA()
	unlockA() // a second release is a no-op
	unlockAgain := <-acquired
	require.Equal(t, 2, locks.size())

	unlockAgain()
	unlockB()
	require.Zero(t, locks.size())

	for i := range 1000 {
		locks.lock(fmt.Sprintf("client-%d", i))()
	}
	require.Zero(t, locks.size(), "entries for distinct clients must not accumulate")
}
