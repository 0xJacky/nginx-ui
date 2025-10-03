package user

import (
	"context"
	"crypto/rand"
	"math/big"
	"os"
	"path"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	sqlite "github.com/uozi-tech/cosy-driver-sqlite"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
	"github.com/urfave/cli/v3"
	"golang.org/x/crypto/bcrypt"
)

func generateRandomPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+"
	password := make([]byte, length)
	charsetLength := big.NewInt(int64(len(charset)))

	for i := 0; i < length; i++ {
		randomIndex, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			return "", err
		}
		password[i] = charset[randomIndex.Int64()]
	}
	return string(password), nil
}

func ResetInitUserPassword(ctx context.Context, command *cli.Command) error {
	confPath := command.String("config")
	settings.Init(confPath)

	cSettings.ServerSettings.RunMode = gin.ReleaseMode

	logger.Init(cSettings.ServerSettings.RunMode)

	logger.Infof("confPath: %s", confPath)

	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		return ErrConfigNotFound
	}

	dbPath := path.Join(path.Dir(confPath), settings.DatabaseSettings.Name+".db")
	logger.Infof("dbPath: %s", dbPath)
	// check if db file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return ErrDBFileNotFound
	}

	db := cosy.InitDB(sqlite.Open(path.Dir(cSettings.ConfPath), settings.DatabaseSettings))
	model.Use(db)
	query.Init(db)

	u := query.User
	user, err := u.FirstByID(1)
	if err != nil {
		return ErrInitUserNotExists
	}

	pwd, err := generateRandomPassword(12)
	if err != nil {
		return err
	}

	pwdBytes, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = u.Where(u.ID.Eq(1)).Updates(&model.User{
		Password: string(pwdBytes),
	})
	if err != nil {
		return err
	}

	a := query.AuthToken
	_, _ = a.Where(a.UserID.Eq(1)).Delete()

	logger.Infof("User: %s, Password: %s", user.Name, pwd)
	return nil
}
