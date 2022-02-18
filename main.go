package main

import (
	"context"
	"flag"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/router"
	"github.com/0xJacky/Nginx-UI/server/settings"
	tool2 "github.com/0xJacky/Nginx-UI/server/tool"
	"log"
	"mime"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Hack: fix wrong Content Type of .js file on some OS platforms
	// See https://github.com/golang/go/issues/32350
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")

	var confPath string
	flag.StringVar(&confPath, "config", "./app.ini", "Specify the configuration file")
	flag.Parse()

	settings.Init(confPath)
	model.Init()

	srv := &http.Server{
		Addr:    ":" + settings.ServerSettings.HttpPort,
		Handler: router.InitRouter(),
	}

	log.Printf("nginx config dir path: %s", tool2.GetNginxConfPath(""))

	go tool2.AutoCert()

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Listen for the interrupt signal.
	<-ctx.Done()

	// Restore default behavior on the interrupt signal and notify user of shutdown.
	stop()
	log.Println("shutting down gracefully, press Ctrl+C again to force")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")

}
