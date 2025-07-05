package cert

import (
	"context"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
)

// InitRegister init the default user for acme
func InitRegister(ctx context.Context) {
	email := settings.CertSettings.Email
	if settings.CertSettings.Email == "" {
		return
	}
	caDir := settings.CertSettings.GetCADir()
	u := query.AcmeUser

	_, err := u.Where(u.Email.Eq(email),
		u.CADir.Eq(caDir)).First()

	if err == nil {
		return
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error(err)
		return
	}

	// Create a new user
	user := &model.AcmeUser{
		Name:  "System Initial User",
		Email: email,
		CADir: caDir,
	}

	err = user.Register()
	if err != nil {
		logger.Error(err)
		return
	}

	err = u.Create(user)
	if err != nil {
		logger.Error(err)
		return
	}

	logger.Info("ACME Default User registered")
}

func GetDefaultACMEUser() (user *model.AcmeUser, err error) {
	u := query.AcmeUser
	user, err = u.Where(u.Email.Eq(settings.CertSettings.Email),
		u.CADir.Eq(settings.CertSettings.GetCADir())).First()

	if err != nil {
		err = errors.Wrap(err, "get default user error")
		return
	}

	return
}
