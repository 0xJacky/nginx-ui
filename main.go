package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"syscall"

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

func Program(ctx context.Context, confPath string) func(l []net.Listener) error {
	return func(l []net.Listener) error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		listener := l[0]

		cosy.RegisterMigrationsBeforeAutoMigrate(migrate.BeforeAutoMigrate)

		cosy.RegisterModels(model.GenerateAllModel()...)

		cosy.RegisterMigration(migrate.Migrations)

		cosy.RegisterInitFunc(func() {
			kernel.Boot(ctx)
			router.InitRouter()
		})

		// Initialize settings package
		settings.Init(confPath)

		// Set gin mode
		gin.SetMode(cSettings.ServerSettings.RunMode)

		// Initialize logger package
		logger.Init(cSettings.ServerSettings.RunMode)
		defer logger.Sync()
		defer logger.Info("Server exited")

		// Gin router initialization
		cRouter.Init()

		// Kernel boot
		cKernel.Boot(ctx)

		srv := &http.Server{
			Handler: cRouter.GetEngine(),
		}

		// defer Shutdown to wait for ongoing requests to be served before returning
		defer srv.Shutdown(ctx)

		var err error
		if cSettings.ServerSettings.EnableHTTPS {
			// Load TLS certificate
			err = cert.LoadServerTLSCertificate()
			if err != nil {
				logger.Fatalf("Failed to load TLS certificate: %v", err)
				return err
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
			return srv.Serve(tlsListener)
		} else {
			logger.Info("Starting HTTP server")
			return srv.Serve(listener)
		}
	}
}

//go:generate go generate ./cmd/...
func main() {
	appCmd := cmd.NewAppCmd()

	confPath := appCmd.String("config")
	settings.Init(confPath)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	err := risefront.New(ctx, risefront.Config{
		Run:       Program(ctx, confPath),
		Name:      "nginx-ui",
		Addresses: []string{fmt.Sprintf("%s:%d", cSettings.ServerSettings.Host, cSettings.ServerSettings.Port)},
		LogHandler: func(loglevel risefront.LogLevel, kind string, args ...interface{}) {
			switch loglevel {
			case risefront.DebugLevel:
				logger.Debugf(kind, args...)
			case risefront.InfoLevel:
				logger.Infof(kind, args...)
			case risefront.WarnLevel:
				logger.Warnf(kind, args...)
			case risefront.ErrorLevel:
				switch args[0].(type) {
				case error:
					if errors.Is(args[0].(error), net.ErrClosed) {
						return
					}
					logger.Errorf(kind, fmt.Errorf("%v", args[0].(error)))
				default:
					logger.Errorf(kind, args...)
				}
			case risefront.FatalLevel:
				logger.Fatalf(kind, args...)
			case risefront.PanicLevel:
				logger.Panicf(kind, args...)
			default:
				logger.Errorf(kind, args...)
			}
		},
	})
	if err != nil && !errors.Is(err, context.DeadlineExceeded) &&
		!errors.Is(err, context.Canceled) &&
		!errors.Is(err, net.ErrClosed) {
		logger.Error(err)
	}
}
