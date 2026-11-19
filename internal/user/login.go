package user

import (
	"errors"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func Login(name string, password string) (user *model.User, err error) {
	u := query.User

	user, err = u.Where(u.Name.Eq(name)).First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPasswordIncorrect
	}
	if err != nil {
		return nil, err
	}

	// if the user is not initialized, return error
	if user.Password == "" {
		return nil, ErrPasswordIncorrect
	}

	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrPasswordIncorrect
	}

	if !user.Status {
		return nil, ErrUserBanned
	}

	return
}

// loginRecordMutex serializes the short database writes of logins: BanIP and
// the transaction that issues a login token. ban_ips has no key on ip, so two
// concurrent first failures could otherwise each open a window of their own.
// The token transaction reads before it writes, and on SQLite such a
// transaction fails with "database is locked" at once instead of waiting when
// another connection is writing, so failed logins of other clients could
// otherwise make a correct login fail. It is never held across password
// hashing or the failure delay.
var loginRecordMutex sync.Mutex

// BanIP records a failed login attempt for ip. Attempts accumulate in one
// window of BanThresholdMinutes that starts with the first failure, and the
// client is banned while that window holds MaxAttempts or more. A failure
// after the window has ended starts a new window from one.
func BanIP(ip string) {
	loginRecordMutex.Lock()
	defer loginRecordMutex.Unlock()

	b := query.BanIP
	now := time.Now().Unix()

	// Drop the client's ended windows first. Left in place, an ended row was
	// the one read back, so every later failure opened a new row at one
	// attempt and the ban never fired again for that client.
	if _, err := b.Where(b.IP.Eq(ip), b.ExpiredAt.Lt(now)).Delete(); err != nil {
		return
	}

	// Count the failure against the active window. Databases written before
	// this fix can hold several active rows for one client; incrementing all
	// of them lets any of them reach MaxAttempts.
	result, err := b.Where(b.IP.Eq(ip), b.ExpiredAt.Gte(now)).UpdateSimple(b.Attempts.Add(1))
	if err != nil || result.RowsAffected > 0 {
		return
	}

	_ = b.Create(&model.BanIP{
		IP:        ip,
		Attempts:  1,
		ExpiredAt: now + int64(settings.AuthSettings.BanThresholdMinutes*60),
	})
}

// ClearBanIP forgets the failed login attempts of ip after it logged in.
func ClearBanIP(ip string) {
	loginRecordMutex.Lock()
	defer loginRecordMutex.Unlock()

	b := query.BanIP
	_, _ = b.Where(b.IP.Eq(ip)).Delete()
}
