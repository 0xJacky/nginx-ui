package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cmd"
	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/router"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/jpillora/overseer"
	"github.com/uozi-tech/cosy"
	cKernel "github.com/uozi-tech/cosy/kernel"
	"github.com/uozi-tech/cosy/logger"
	cRouter "github.com/uozi-tech/cosy/router"
	cSettings "github.com/uozi-tech/cosy/settings"
)

//go:generate go run cmd/version/generate.go

func Program(confPath string) func(state overseer.State) {
	return func(state overseer.State) {
		defer logger.Sync()
		defer logger.Info("Server exited")
		cosy.RegisterModels(model.GenerateAllModel()...)

		cosy.RegisterInitFunc(kernel.Boot, router.InitRouter)

		// Initialize settings package
		settings.Init(confPath)

		// Set gin mode
		gin.SetMode(cSettings.ServerSettings.RunMode)

		// Initialize logger package
		logger.Init(cSettings.ServerSettings.RunMode)
		defer logger.Sync()

		if state.Listener == nil {
			return
		}
		// Gin router initialization
		cRouter.Init()

		// Kernel boot
		cKernel.Boot()

		addr := fmt.Sprintf("%s:%d", cSettings.ServerSettings.Host, cSettings.ServerSettings.Port)
		srv := &http.Server{
			Addr:    addr,
			Handler: cRouter.GetEngine(),
		}
		if err := srv.Serve(state.Listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("listen: %s\n", err)
		}
	}
}

func main() {
	appCmd := cmd.NewAppCmd()

	confPath := appCmd.String("config")
	settings.Init(confPath)
	overseer.Run(overseer.Config{
		Program:          Program(confPath),
		Address:          fmt.Sprintf("%s:%d", cSettings.ServerSettings.Host, cSettings.ServerSettings.Port),
		TerminateTimeout: 5 * time.Second,
	})
}
