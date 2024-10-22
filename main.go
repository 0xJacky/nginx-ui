package main

import (
	"flag"
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/kernal"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/router"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/jpillora/overseer"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
	"time"
)

func Program(confPath string) func(state overseer.State) {
	return func(state overseer.State) {
		defer logger.Sync()

		cosy.RegisterModels(model.GenerateAllModel()...)

		cosy.RegisterAsyncFunc(kernal.Boot, router.InitRouter)

		if state.Listener != nil {
			cosy.SetListener(state.Listener)

			cosy.Boot(confPath)

			logger.Infof("Nginx configuration directory: %s", nginx.GetConfPath())
		}
		logger.Info("Server exited")
	}
}

func main() {
	var confPath string
	flag.StringVar(&confPath, "config", "app.ini", "Specify the configuration file")
	flag.Parse()

	settings.Migrate(confPath)
	cSettings.Init(confPath)

	overseer.Run(overseer.Config{
		Program:          Program(confPath),
		Address:          fmt.Sprintf("%s:%d", cSettings.ServerSettings.Host, cSettings.ServerSettings.Port),
		TerminateTimeout: 5 * time.Second,
	})
}
