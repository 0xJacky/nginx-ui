package user

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
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

// loginRecordMutex serializes the short database writes of logins: BanIP,
// ClearBanIP, and the transactions that issue a login token, consume a
// recovery code and save MFA credentials enrolled before login. ban_ips has
// no key on ip, so two concurrent first failures could otherwise each open a
// window of their own. The transactions read before they write, and on SQLite
// such a transaction fails with "database is locked" at once instead of
// waiting when another connection is writing, so failed logins of other
// clients could otherwise make a correct login fail. It is never held across
// password hashing or the failure delay.
var loginRecordMutex sync.Mutex

// IsDatabaseLocked reports whether err is SQLite refusing a statement because
// another connection holds the lock. Such an error says nothing about the
// credentials that were being checked.
func IsDatabaseLocked(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") ||
		strings.Contains(message, "database table is locked")
}

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
		logger.Error("[BanIP] drop ended windows:", err)
		return
	}

	// Count the failure against the active window. Databases written before
	// this fix can hold several active rows for one client; incrementing all
	// of them lets any of them reach MaxAttempts.
	result, err := b.Where(b.IP.Eq(ip), b.ExpiredAt.Gte(now)).UpdateSimple(b.Attempts.Add(1))
	if err != nil {
		logger.Error("[BanIP] count failed attempt:", err)
		return
	}
	if result.RowsAffected > 0 {
		return
	}

	if err := b.Create(&model.BanIP{
		IP:        ip,
		Attempts:  1,
		ExpiredAt: now + int64(settings.AuthSettings.BanThresholdMinutes*60),
	}); err != nil {
		logger.Error("[BanIP] open window:", err)
	}
}

// ClearBanIP forgets the failed login attempts of ip after it logged in.
func ClearBanIP(ip string) {
	loginRecordMutex.Lock()
	defer loginRecordMutex.Unlock()

	b := query.BanIP
	if _, err := b.Where(b.IP.Eq(ip)).Delete(); err != nil {
		logger.Error("[ClearBanIP]", err)
	}
}
