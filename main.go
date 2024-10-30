package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/cmd"

	"code.pfad.fr/risefront"
	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/0xJacky/Nginx-UI/internal/migrate"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/router"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy"
	cKernel "github.com/uozi-tech/cosy/kernel"
	"github.com/uozi-tech/cosy/logger"
	cRouter "github.com/uozi-tech/cosy/router"
	cSettings "github.com/uozi-tech/cosy/settings"
)

func Program(confPath string) func(l []net.Listener) error {
	return func(l []net.Listener) error {
		listener := l[0]
		defer logger.Sync()
		defer logger.Info("Server exited")

		cosy.RegisterMigrationsBeforeAutoMigrate(migrate.BeforeAutoMigrate)

		cosy.RegisterModels(model.GenerateAllModel()...)

		cosy.RegisterMigration(migrate.Migrations)

		cosy.RegisterInitFunc(kernel.Boot, router.InitRouter)

		// Initialize settings package
		settings.Init(confPath)

		// Set gin mode
		gin.SetMode(cSettings.ServerSettings.RunMode)

		// Initialize logger package
		logger.Init(cSettings.ServerSettings.RunMode)
		defer logger.Sync()

		// Gin router initialization
		cRouter.Init()

		// Kernel boot
		cKernel.Boot()

		srv := &http.Server{
			Handler: cRouter.GetEngine(),
		}
		// defer Shutdown to wait for ongoing requests to be served before returning
		defer func(srv *http.Server, ctx context.Context) {
			err := srv.Shutdown(ctx)
			if err != nil {
				logger.Fatal(err)
			}
		}(srv, context.Background())
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
				MinVersion: tls.VersionTLS12,
			}

			srv.TLSConfig = tlsConfig

			logger.Info("Starting HTTPS server")
			tlsListener := tls.NewListener(listener, tlsConfig)
			err = srv.Serve(tlsListener)
		} else {
			logger.Info("Starting HTTP server")
			err = srv.Serve(listener)
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := risefront.New(ctx, risefront.Config{
		Run:       Program(confPath),
		Debug:     cSettings.ServerSettings.RunMode == gin.DebugMode,
		Addresses: []string{fmt.Sprintf("%s:%d", cSettings.ServerSettings.Host, cSettings.ServerSettings.Port)},
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		logger.Fatal(err)
	}
}
