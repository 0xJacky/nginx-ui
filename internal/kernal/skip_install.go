package kernal

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/caarlos0/env/v11"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type predefinedUser struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func skipInstall() {
	logger.Info("Skip installation mode enabled")

	if settings.ServerSettings.JwtSecret == "" {
		settings.ServerSettings.JwtSecret = uuid.New().String()
	}

	if settings.ServerSettings.NodeSecret == "" {
		settings.ServerSettings.NodeSecret = uuid.New().String()
		logger.Infof("NodeSecret: %s", settings.ServerSettings.NodeSecret)
	}

	err := settings.Save()
	if err != nil {
		logger.Fatal(err)
	}
}

func registerPredefinedUser() {
	// when skip installation mode is enabled, the predefined user will be created
	if !settings.ServerSettings.SkipInstallation {
		return
	}
	pUser := &predefinedUser{}

	err := env.ParseWithOptions(pUser, env.Options{
		Prefix:                "NGINX_UI_PREDEFINED_USER_",
		UseFieldNameByDefault: true,
	})

	if err != nil {
		logger.Fatal(err)
	}

	u := query.Auth

	_, err = u.First()

	// Only effect when there is no user in the database
	if !errors.Is(err, gorm.ErrRecordNotFound) || pUser.Name == "" || pUser.Password == "" {
		return
	}

	// Create a new user with the predefined name and password
	pwd, _ := bcrypt.GenerateFromPassword([]byte(pUser.Password), bcrypt.DefaultCost)

	err = u.Create(&model.Auth{
		Name:     pUser.Name,
		Password: string(pwd),
	})

	if err != nil {
		logger.Error(err)
	}
}
