package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"

	"code.pfad.fr/risefront"
	"github.com/0xJacky/Nginx-UI/internal/kernel"
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
		defer logger.Sync()
		defer logger.Info("Server exited")
		cosy.RegisterModels(model.GenerateAllModel()...)

		cosy.RegisterAsyncFunc(kernel.Boot, router.InitRouter)

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
		return srv.Serve(l[0])
	}
}

func main() {
	var confPath string
	flag.StringVar(&confPath, "config", "app.ini", "Specify the configuration file")
	flag.Parse()

	settings.Init(confPath)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := risefront.New(ctx, risefront.Config{
		Run:       Program(confPath),
		Name:      "nginx-ui",
		Addresses: []string{fmt.Sprintf("%s:%d", cSettings.ServerSettings.Host, cSettings.ServerSettings.Port)},
		ErrorHandler: func(kind string, err error) {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logger.Error(kind, err)
		},
	})
	if err != nil && !errors.Is(err, context.DeadlineExceeded) &&
		!errors.Is(err, context.Canceled) &&
		!errors.Is(err, net.ErrClosed) {
		logger.Error(err)
	}
}
