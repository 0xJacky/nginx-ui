package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert"
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
		var err error
		if cSettings.ServerSettings.EnableHTTPS {
			// Load TLS certificate
			err = cert.LoadServerTLSCertificate()
			if err != nil {
				logger.Fatalf("Failed to load TLS certificate: %v", err)
				return
			}

			tlsConfig := &tls.Config{
				GetCertificate: func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
					return cert.GetServerTLSCertificate()
				},
			}

			srv.TLSConfig = tlsConfig

			logger.Info("Starting HTTPS server")
			err = srv.ServeTLS(state.Listener, "", "")
		} else {
			logger.Info("Starting HTTP server")
			err = srv.Serve(state.Listener)
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
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
