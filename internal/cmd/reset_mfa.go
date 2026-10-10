package cmd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
	sqlite "github.com/uozi-tech/cosy-driver-sqlite"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
	"github.com/urfave/cli/v3"
)

var ResetMFACommand = &cli.Command{
	Name:  "reset-mfa",
	Usage: "Reset a user's MFA credentials and sessions without changing policy",
	Flags: []cli.Flag{&cli.StringFlag{Name: "username", Required: true}},
	Action: func(_ context.Context, command *cli.Command) error {
		config := command.String("config")
		if _, err := os.Stat(config); err != nil {
			return err
		}
		settings.Init(config)
		logger.Init("release")
		directory := filepath.Dir(cSettings.ConfPath)
		if _, err := os.Stat(filepath.Join(directory, settings.DatabaseSettings.Name+".db")); err != nil {
			return err
		}
		db := cosy.InitDB(sqlite.Open(directory, settings.DatabaseSettings))
		model.Use(db)
		query.Init(db)
		if err := db.AutoMigrate(&model.User{}); err != nil {
			return err
		}
		u, err := user.GetUser(command.String("username"))
		if err != nil {
			return err
		}
		if err := user.ResetMFA(u.ID); err != nil {
			return err
		}
		logger.Infof("MFA reset for user %s (ID %d); the MFA requirement is unchanged", u.Name, u.ID)
		return nil
	},
}
