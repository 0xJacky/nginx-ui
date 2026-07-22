package kernel

import (
	"context"
	"errors"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/caarlos0/env/v11"
	"github.com/google/uuid"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type predefinedUser struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func skipInstall() {
	logger.Info("Skip installation mode enabled")

	var nodeSecret string

	err := settings.Update(func() {
		if cSettings.AppSettings.JwtSecret == "" {
			cSettings.AppSettings.JwtSecret = uuid.New().String()
		}

		if settings.NodeSettings.Secret == "" {
			nodeSecret = uuid.New().String()
			settings.NodeSettings.Secret = nodeSecret
		}
	})
	if err != nil {
		logger.Fatal(err)
	}

	if nodeSecret != "" {
		logger.Infof("Secret: %s", nodeSecret)
	}
}

func registerPredefinedUser(ctx context.Context) {
	// when skip installation mode is enabled, the predefined user will be created
	if !settings.NodeSettings.SkipInstallation {
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

	// No predefined credentials configured, nothing to do
	if pUser.Name == "" || pUser.Password == "" {
		return
	}

	u := query.User

	user, err := u.First()

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error(err)
		return
	}

	pwd, _ := bcrypt.GenerateFromPassword([]byte(pUser.Password), bcrypt.DefaultCost)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create the initial user when the database is empty
		err = u.Create(&model.User{
			Model: model.Model{
				ID: 1,
			},
			Name:     pUser.Name,
			Password: string(pwd),
		})
	} else if user.Password == "" {
		// The initial user already exists but has no password, apply the
		// predefined credentials. This happens when InitUser created the empty
		// admin user before registerPredefinedUser runs.
		_, err = u.Where(u.ID.Eq(1)).Updates(&model.User{
			Name:     pUser.Name,
			Password: string(pwd),
		})
	}

	if err != nil {
		logger.Error(err)
	}
}
