package user

import (
	"errors"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"golang.org/x/crypto/bcrypt"
	"time"
)

var (
	ErrPasswordIncorrect = errors.New("password incorrect")
	ErrUserBanned        = errors.New("user banned")
)

func Login(name string, password string) (user *model.User, err error) {
	u := query.User

	user, err = u.Where(u.Name.Eq(name)).First()
	if err != nil {
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

func BanIP(ip string) {
	b := query.BanIP
	banIP, err := b.Where(b.IP.Eq(ip)).First()
	if err != nil || banIP.ExpiredAt <= time.Now().Unix() {
		_ = b.Create(&model.BanIP{
			IP:        ip,
			Attempts:  1,
			ExpiredAt: time.Now().Unix() + int64(settings.AuthSettings.BanThresholdMinutes*60),
		})
		return
	}
	_, _ = b.Where(b.IP.Eq(ip)).UpdateSimple(b.Attempts.Add(1))
}
