package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os/signal"
	"syscall"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/cmd"
	"github.com/0xJacky/Nginx-UI/internal/process"

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

		// Get the HTTP handler from Cosy router
		handler := cRouter.GetEngine()

		// Configure TLS if HTTPS is enabled
		var tlsConfig *tls.Config
		if cSettings.ServerSettings.EnableHTTPS {
			// Load TLS certificate
			err := cert.LoadServerTLSCertificate()
			if err != nil {
				logger.Fatalf("Failed to load TLS certificate: %v", err)
				return err
			}

			// Configure ALPN protocols based on settings
			// Protocol negotiation priority is fixed: h3 -> h2 -> h1
			var nextProtos []string
			if cSettings.ServerSettings.EnableH3 {
				nextProtos = append(nextProtos, "h3")
			}
			if cSettings.ServerSettings.EnableH2 {
				nextProtos = append(nextProtos, "h2")
			}
			// HTTP/1.1 is always supported as fallback
			nextProtos = append(nextProtos, "http/1.1")

			tlsConfig = &tls.Config{
				GetCertificate: func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
					return cert.GetServerTLSCertificate()
				},
				MinVersion: tls.VersionTLS12,
				NextProtos: nextProtos,
			}
		}

		// Create and initialize the server factory
		serverFactory := cKernel.NewServerFactory(handler, tlsConfig)
		if err := serverFactory.Initialize(); err != nil {
			logger.Fatalf("Failed to initialize server factory: %v", err)
			return err
		}

		go func() {
			// Wait for context cancellation
			<-ctx.Done()

			// Graceful shutdown
			logger.Info("Shutting down servers...")
			if err := serverFactory.Shutdown(ctx); err != nil {
				logger.Errorf("Error during server shutdown: %v", err)
			}
		}()

		// Start the servers
		if err := serverFactory.Start(ctx, listener); err != nil {
			logger.Fatalf("Failed to start servers: %v", err)
			return err
		}

		<-ctx.Done()

		// Graceful shutdown
		logger.Info("Shutting down servers...")
		if err := serverFactory.Shutdown(ctx); err != nil {
			logger.Errorf("Error during server shutdown: %v", err)
		}

		return nil
	}
}

//go:generate go generate ./cmd/...
func main() {
	appCmd := cmd.NewAppCmd()

	confPath := appCmd.String("config")
	settings.Init(confPath)

	mainCtx, mainCancel := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	defer mainCancel()

	pidPath := appCmd.String("pidfile")
	if pidPath != "" {
		if err := process.WritePIDFile(pidPath); err != nil {
			logger.Fatalf("Failed to write PID file: %v", err)
		}
		defer process.RemovePIDFile(pidPath)
	}

	var programCancel context.CancelFunc

	err := risefront.New(mainCtx, risefront.Config{
		Run: func(l []net.Listener) error {
			// Create a new context for the program itself, derived from the main context.
			programCtx, cancel := context.WithCancel(mainCtx)
			// Store the cancel function so the Shutdown callback can use it.
			programCancel = cancel
			return Program(programCtx, confPath)(l)
		},
		Shutdown: func() {
			// This is called by risefront.Restart() to shut down the old program.
			if programCancel != nil {
				programCancel()
			}
		},
		Name:      "nginx-ui",
		Addresses: []string{fmt.Sprintf("%s:%d", cSettings.ServerSettings.Host, cSettings.ServerSettings.Port)},
		LogHandler: func(loglevel risefront.LogLevel, kind string, args ...any) {
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
