package user

import (
    "errors"
    "github.com/0xJacky/Nginx-UI/model"
    "github.com/0xJacky/Nginx-UI/query"
    "golang.org/x/crypto/bcrypt"
)

var (
	ErrPasswordIncorrect = errors.New("password incorrect")
	ErrUserBanned        = errors.New("user banned")
)

func Login(name string, password string) (user *model.Auth, err error) {
	u := query.Auth

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
