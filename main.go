package main

import (
	"flag"
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/kernal"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/router"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/jpillora/overseer"
	"net/http"
	"time"
)

func Program(state overseer.State) {
	defer logger.Sync()

	logger.Infof("Nginx configuration directory: %s", nginx.GetConfPath())

	kernal.Boot()

	if state.Listener != nil {
		err := http.Serve(state.Listener, router.InitRouter())
		if err != nil {
			logger.Error(err)
		}
	}

	logger.Info("Server exited")
}

func main() {
	var confPath string
	flag.StringVar(&confPath, "config", "app.ini", "Specify the configuration file")
	flag.Parse()

	settings.Init(confPath)

	gin.SetMode(settings.ServerSettings.RunMode)

	overseer.Run(overseer.Config{
		Program:          Program,
		Address:          fmt.Sprintf("%s:%s", settings.ServerSettings.HttpHost, settings.ServerSettings.HttpPort),
		TerminateTimeout: 5 * time.Second,
	})
}
