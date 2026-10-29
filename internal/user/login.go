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

// banIPMutex serializes BanIP. ban_ips has no key on ip, so two concurrent
// first failures could otherwise each open a window of their own.
var banIPMutex sync.Mutex

// BanIP records a failed login attempt for ip. Attempts accumulate in one
// window of BanThresholdMinutes that starts with the first failure, and the
// client is banned while that window holds MaxAttempts or more. A failure
// after the window has ended starts a new window from one.
func BanIP(ip string) {
	banIPMutex.Lock()
	defer banIPMutex.Unlock()

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
