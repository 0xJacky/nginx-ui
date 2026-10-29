package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	apisettings "github.com/0xJacky/Nginx-UI/api/settings"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	verifyAttackerIP = "198.51.100.7"
	verifySpoofedIP  = "203.0.113.9"
	verifyOtherIP    = "192.0.2.44"
	verifyPassword   = "correct horse battery staple"
)

// stepUpUserIDs hands every step-up test its own account. The step-up
// failure counter lives in process memory, so reusing an ID would carry a
// lock from one test, or one -count run, into the next.
var stepUpUserIDs atomic.Uint64

func init() {
	stepUpUserIDs.Store(1000)
}

func setupLoginBanFixture(t *testing.T, trustedProxies []string) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	originalJWTSecret := cSettings.AppSettings.JwtSecret
	cSettings.AppSettings.JwtSecret = "test-secret"
	t.Cleanup(func() { cSettings.AppSettings.JwtSecret = originalJWTSecret })

	// Defaults from settings/auth.go: MaxAttempts 10, BanThresholdMinutes 10.
	require.Equal(t, 10, settings.AuthSettings.MaxAttempts)
	require.Equal(t, 10, settings.AuthSettings.BanThresholdMinutes)

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}, &model.BanIP{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	hash, err := bcrypt.GenerateFromPassword([]byte(verifyPassword), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.User{
		Model: model.Model{ID: 1}, Name: "admin", Password: string(hash), Status: true, Language: "en",
	}).Error)

	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(trustedProxies))
	router.POST("/login", Login)
	return router, db
}

// enableOTP gives the admin account a TOTP secret and returns it.
func enableOTP(t *testing.T, db *gorm.DB) string {
	t.Helper()
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "verify", AccountName: "admin"})
	require.NoError(t, err)
	encrypted, err := crypto.AesEncrypt([]byte(key.Secret()))
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("otp_secret", encrypted).Error)
	return key.Secret()
}

func currentOTP(t *testing.T, secret string) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	return code
}

// wrongOTP returns the i-th six-digit code that is not accepted for secret
// right now. totp.Validate allows one period of skew, so the codes of the
// neighbouring periods are skipped as well.
func wrongOTP(t *testing.T, secret string, i int) string {
	t.Helper()
	valid := map[string]bool{}
	for _, offset := range []time.Duration{-30 * time.Second, 0, 30 * time.Second} {
		code, err := totp.GenerateCode(secret, time.Now().Add(offset))
		require.NoError(t, err)
		valid[code] = true
	}
	for n := i; ; n++ {
		code := fmt.Sprintf("%06d", n)
		if !valid[code] {
			return code
		}
	}
}

func postLoginRequest(router http.Handler, remoteAddr, xff string, body LoginUser) (int, LoginResponse) {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr + ":40000"
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var resp LoginResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return rec.Code, resp
}

func postLogin(router http.Handler, remoteAddr, xff, password string) (int, LoginResponse) {
	return postLoginRequest(router, remoteAddr, xff, LoginUser{Name: "admin", Password: password})
}

// postLoginOTP sends the correct password with the given OTP. A wrong OTP is
// a failed attempt without the random delay of a wrong password, which keeps
// handler-level tests fast.
func postLoginOTP(router http.Handler, remoteAddr, otp string) (int, LoginResponse) {
	return postLoginRequest(router, remoteAddr, "", LoginUser{Name: "admin", Password: verifyPassword, OTP: otp})
}

func bannedLikeLogin(t *testing.T, ip string) int64 {
	t.Helper()
	b := query.BanIP
	n, err := b.Where(b.IP.Eq(ip), b.ExpiredAt.Gte(time.Now().Unix()),
		b.Attempts.Gte(settings.AuthSettings.MaxAttempts)).Count()
	require.NoError(t, err)
	return n
}

func banRows(t *testing.T, db *gorm.DB, ip string) []model.BanIP {
	t.Helper()
	var rows []model.BanIP
	require.NoError(t, db.Where("ip = ?", ip).Find(&rows).Error)
	return rows
}

// expireBans moves every ban window of ip into the past, as if
// BanThresholdMinutes had elapsed.
func expireBans(t *testing.T, db *gorm.DB, ip string) {
	t.Helper()
	require.NoError(t, db.Model(&model.BanIP{}).Where("ip = ?", ip).
		Update("expired_at", time.Now().Unix()-1).Error)
}

// The 10th failed login from one client IP bans it; afterwards even the
// correct password is refused with 429 / code 4291.
func TestVerifyLoginBansClientAfterDefaultMaxAttempts(t *testing.T) {
	router, _ := setupLoginBanFixture(t, nil)

	for range settings.AuthSettings.MaxAttempts - 1 {
		internaluser.BanIP(verifyAttackerIP) // same call Login makes on failure
	}
	status, _ := postLogin(router, verifyAttackerIP, "", "wrong") // 10th failure via the handler
	require.NotEqual(t, http.StatusOK, status)
	require.EqualValues(t, 1, bannedLikeLogin(t, verifyAttackerIP))

	status, resp := postLogin(router, verifyAttackerIP, "", verifyPassword)
	require.Equal(t, http.StatusTooManyRequests, status)
	require.Equal(t, ErrMaxAttempts, resp.Code)
}

// Without configured trusted proxies a client cannot pick its identity with
// X-Forwarded-For; behind the bundled proxy (loopback trusted) the address
// appended by $proxy_add_x_forwarded_for wins over a spoofed prefix.
func TestVerifyLoginClientIdentityIgnoresSpoofedForwardedFor(t *testing.T) {
	for _, tc := range []struct {
		name           string
		trustedProxies []string
		remote, xff    string
	}{
		{"direct, no trusted proxies", nil, verifyAttackerIP, verifySpoofedIP},
		{"bundled proxy", []string{"127.0.0.1", "::1"}, "127.0.0.1", verifySpoofedIP + ", " + verifyAttackerIP},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, _ := setupLoginBanFixture(t, tc.trustedProxies)
			for range settings.AuthSettings.MaxAttempts {
				internaluser.BanIP(verifyAttackerIP)
			}
			status, resp := postLogin(router, tc.remote, tc.xff, verifyPassword)
			require.Equal(t, http.StatusTooManyRequests, status)
			require.Equal(t, ErrMaxAttempts, resp.Code)
		})
	}
}

// After a client's first ban window has expired, BanIP must start a fresh
// window that can reach MaxAttempts again.
func TestVerifyBanIPReachesThresholdAfterPreviousWindowExpired(t *testing.T) {
	_, db := setupLoginBanFixture(t, nil)

	require.NoError(t, db.Create(&model.BanIP{
		IP: verifyAttackerIP, Attempts: settings.AuthSettings.MaxAttempts, ExpiredAt: time.Now().Unix() - 1,
	}).Error)

	for range 3 * settings.AuthSettings.MaxAttempts {
		internaluser.BanIP(verifyAttackerIP)
	}

	rows := banRows(t, db, verifyAttackerIP)
	t.Logf("ban_ips rows for %s after %d failures: %d", verifyAttackerIP, 3*settings.AuthSettings.MaxAttempts, len(rows))
	require.EqualValues(t, 1, bannedLikeLogin(t, verifyAttackerIP),
		"client was never banned again after its previous ban window expired")
	require.Len(t, rows, 1, "the expired window must be replaced, not shadow the new one")
}

// Attack side, end to end: once the first ban has expired, failures through
// the login handler accumulate again and ban the client a second time.
func TestLoginBansAgainAfterPreviousBanExpired(t *testing.T) {
	router, db := setupLoginBanFixture(t, nil)
	secret := enableOTP(t, db)

	for i := range settings.AuthSettings.MaxAttempts {
		postLoginOTP(router, verifyAttackerIP, wrongOTP(t, secret, i))
	}
	status, resp := postLoginOTP(router, verifyAttackerIP, currentOTP(t, secret))
	require.Equal(t, http.StatusTooManyRequests, status)
	require.Equal(t, ErrMaxAttempts, resp.Code)

	expireBans(t, db, verifyAttackerIP)

	for i := range settings.AuthSettings.MaxAttempts {
		status, _ := postLoginOTP(router, verifyAttackerIP, wrongOTP(t, secret, 100+i))
		require.NotEqual(t, http.StatusOK, status)
	}
	status, resp = postLoginOTP(router, verifyAttackerIP, currentOTP(t, secret))
	require.Equal(t, http.StatusTooManyRequests, status, "the second window never banned the client")
	require.Equal(t, ErrMaxAttempts, resp.Code)
	require.Len(t, banRows(t, db, verifyAttackerIP), 1)
}

// Legitimate side: a few typos below the limit do not block the next correct
// login, and the successful login clears the client's counter.
func TestLoginSucceedsAfterFailuresBelowLimit(t *testing.T) {
	router, db := setupLoginBanFixture(t, nil)
	secret := enableOTP(t, db)

	for i := range settings.AuthSettings.MaxAttempts - 1 {
		status, _ := postLoginOTP(router, verifyAttackerIP, wrongOTP(t, secret, i))
		require.NotEqual(t, http.StatusTooManyRequests, status)
	}
	status, resp := postLoginOTP(router, verifyAttackerIP, currentOTP(t, secret))
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, LoginSuccess, resp.Code)
	require.Empty(t, banRows(t, db, verifyAttackerIP))
}

// Legitimate side: a ban lifts once BanThresholdMinutes have passed, and the
// next failure starts a clean window instead of resuming the old count.
func TestLoginBanExpiresAfterThreshold(t *testing.T) {
	router, db := setupLoginBanFixture(t, nil)
	secret := enableOTP(t, db)

	for range settings.AuthSettings.MaxAttempts {
		internaluser.BanIP(verifyAttackerIP)
	}
	status, _ := postLoginOTP(router, verifyAttackerIP, currentOTP(t, secret))
	require.Equal(t, http.StatusTooManyRequests, status)

	expireBans(t, db, verifyAttackerIP)

	internaluser.BanIP(verifyAttackerIP)
	rows := banRows(t, db, verifyAttackerIP)
	require.Len(t, rows, 1)
	require.Equal(t, 1, rows[0].Attempts)
	require.Greater(t, rows[0].ExpiredAt, time.Now().Unix())

	status, resp := postLoginOTP(router, verifyAttackerIP, currentOTP(t, secret))
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, LoginSuccess, resp.Code)
}

// Legitimate side: the banned IP list shows a banned client once, and
// removing it there lets the client log in again.
func TestRemoveBannedIPUnblocksLogin(t *testing.T) {
	router, db := setupLoginBanFixture(t, nil)
	secret := enableOTP(t, db)
	router.GET("/settings/auth/banned_ips", apisettings.GetBanLoginIP)
	router.DELETE("/settings/auth/banned_ip", apisettings.RemoveBannedIP)

	for range settings.AuthSettings.MaxAttempts {
		internaluser.BanIP(verifyAttackerIP)
	}
	internaluser.BanIP(verifyOtherIP)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/auth/banned_ips", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var listed []model.BanIP
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listed))
	require.Len(t, listed, 1)
	require.Equal(t, verifyAttackerIP, listed[0].IP)

	req := httptest.NewRequest(http.MethodDelete, "/settings/auth/banned_ip",
		bytes.NewBufferString(`{"ip":"`+verifyAttackerIP+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, banRows(t, db, verifyAttackerIP))
	require.Len(t, banRows(t, db, verifyOtherIP), 1, "removing one ban must not touch other clients")

	status, resp := postLoginOTP(router, verifyAttackerIP, currentOTP(t, secret))
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, LoginSuccess, resp.Code)
}

// Legitimate side: behind the bundled proxy, clients are still told apart by
// the address nginx appends, so one banned client does not lock out another.
func TestLoginBanBehindTrustedProxyIsPerClient(t *testing.T) {
	router, db := setupLoginBanFixture(t, []string{"127.0.0.1", "::1"})
	secret := enableOTP(t, db)

	for i := range settings.AuthSettings.MaxAttempts {
		postLoginRequest(router, "127.0.0.1", verifyAttackerIP,
			LoginUser{Name: "admin", Password: verifyPassword, OTP: wrongOTP(t, secret, i)})
	}
	require.EqualValues(t, 1, bannedLikeLogin(t, verifyAttackerIP))
	require.Empty(t, banRows(t, db, "127.0.0.1"), "the proxy address must not be the ban key")

	status, _ := postLoginRequest(router, "127.0.0.1", verifyAttackerIP,
		LoginUser{Name: "admin", Password: verifyPassword, OTP: currentOTP(t, secret)})
	require.Equal(t, http.StatusTooManyRequests, status)

	status, resp := postLoginRequest(router, "127.0.0.1", verifyOtherIP,
		LoginUser{Name: "admin", Password: verifyPassword, OTP: currentOTP(t, secret)})
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, LoginSuccess, resp.Code)
}

type stepUpFixture struct {
	router *gin.Engine
	secret string
}

// setupStepUpFixture serves the OTP step-up endpoint for a fresh TOTP account
// that is already logged in, as RequireInteractiveUser leaves it.
func setupStepUpFixture(t *testing.T) stepUpFixture {
	t.Helper()
	_, db := setupLoginBanFixture(t, nil)
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "verify", AccountName: "step-up"})
	require.NoError(t, err)
	encrypted, err := crypto.AesEncrypt([]byte(key.Secret()))
	require.NoError(t, err)
	id := stepUpUserIDs.Add(1)
	u := model.User{
		Model: model.Model{ID: id}, Name: fmt.Sprintf("step-up-%d", id),
		Status: true, Language: "en", OTPSecret: encrypted,
	}
	require.NoError(t, db.Create(&u).Error)

	router := gin.New()
	// Load the account per request, as AuthRequired does. VerifyOTP decrypts
	// the secret in place, so one shared struct would not survive a second call.
	router.Use(func(c *gin.Context) {
		var current model.User
		if err := db.First(&current, id).Error; err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Set("user", &current)
	})
	router.POST("/2fa_secure_session/otp", Start2FASecureSessionByOTP)
	return stepUpFixture{router: router, secret: key.Secret()}
}

func (f stepUpFixture) send(otp, recoveryCode string) int {
	body, _ := json.Marshal(map[string]string{"otp": otp, "recovery_code": recoveryCode})
	req := httptest.NewRequest(http.MethodPost, "/2fa_secure_session/otp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = verifyAttackerIP + ":40000"
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec.Code
}

// Attack side: an authenticated session cannot keep guessing the step-up
// OTP. After MaxAttempts failures every attempt, even the correct code, gets
// 429 until the window ends.
func TestSecureSessionOTPAttemptsAreLimited(t *testing.T) {
	f := setupStepUpFixture(t)

	for i := range settings.AuthSettings.MaxAttempts {
		require.NotEqual(t, http.StatusTooManyRequests, f.send(wrongOTP(t, f.secret, i), ""))
	}
	require.Equal(t, http.StatusTooManyRequests, f.send(wrongOTP(t, f.secret, 500), ""))
	require.Equal(t, http.StatusTooManyRequests, f.send(currentOTP(t, f.secret), ""),
		"a locked step-up must not reveal whether a guess was right")
	require.Equal(t, http.StatusTooManyRequests, f.send("", "not-a-recovery-code"),
		"recovery codes share the step-up limit")
}

// Attack side: concurrent guesses cannot slip past the limit between the
// check and the recorded failure.
func TestSecureSessionOTPLimitHoldsUnderConcurrency(t *testing.T) {
	f := setupStepUpFixture(t)

	var wg sync.WaitGroup
	var answered atomic.Int64
	for i := range 5 * settings.AuthSettings.MaxAttempts {
		code := wrongOTP(t, f.secret, i)
		wg.Go(func() {
			if f.send(code, "") != http.StatusTooManyRequests {
				answered.Add(1)
			}
		})
	}
	wg.Wait()
	require.EqualValues(t, settings.AuthSettings.MaxAttempts, answered.Load())
}

// Legitimate side: a few mistyped codes do not block the correct one, and
// the success resets the counter so later typos start from zero.
func TestSecureSessionOTPAcceptsCorrectCodeAfterTypos(t *testing.T) {
	f := setupStepUpFixture(t)

	for round := range 2 {
		for i := range settings.AuthSettings.MaxAttempts - 1 {
			require.NotEqual(t, http.StatusTooManyRequests, f.send(wrongOTP(t, f.secret, round*100+i), ""))
		}
		require.Equal(t, http.StatusOK, f.send(currentOTP(t, f.secret), ""), "round %d", round)
	}
}

// Legitimate side: one account's step-up lock does not affect another.
func TestSecureSessionOTPLimitIsPerUser(t *testing.T) {
	locked := setupStepUpFixture(t)
	for i := range settings.AuthSettings.MaxAttempts {
		locked.send(wrongOTP(t, locked.secret, i), "")
	}
	require.Equal(t, http.StatusTooManyRequests, locked.send(currentOTP(t, locked.secret), ""))

	other := setupStepUpFixture(t)
	require.Equal(t, http.StatusOK, other.send(currentOTP(t, other.secret), ""))
}
