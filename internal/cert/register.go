package cert

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// InitRegister init the default user for acme
func InitRegister() {
	if settings.ServerSettings.Email == "" {
		return
	}
	u := query.AcmeUser

	_, err := u.Where(u.Email.Eq(settings.ServerSettings.Email),
		u.CADir.Eq(settings.ServerSettings.GetCADir())).First()

	if err == nil {
		return
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error(err)
		return
	}

	// Create a new user
	user := &User{
		AcmeUser: model.AcmeUser{
			Name:  "System Initial User",
			Email: settings.ServerSettings.Email,
			CADir: settings.ServerSettings.GetCADir(),
		},
	}

	err = user.Register()
	if err != nil {
		logger.Error(err)
		return
	}

	err = u.Create(&user.AcmeUser)
	if err != nil {
		logger.Error(err)
		return
	}

	logger.Info("ACME Default User registered")
}
