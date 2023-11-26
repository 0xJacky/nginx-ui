package main

import (
	"flag"
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/boot"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/upgrader"
	"github.com/0xJacky/Nginx-UI/router"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/jpillora/overseer"
	"github.com/jpillora/overseer/fetcher"
	"log"
	"net/http"
)

func Program(state overseer.State) {
	defer logger.Sync()

	logger.Infof("Nginx configuration directory: %s", nginx.GetConfPath())

	boot.Kernel()

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

	r, err := upgrader.GetRuntimeInfo()

	if err != nil {
		log.Fatalln(err)
	}

	overseer.Run(overseer.Config{
		Program:          Program,
		Address:          fmt.Sprintf("%s:%s", settings.ServerSettings.HttpHost, settings.ServerSettings.HttpPort),
		Fetcher:          &fetcher.File{Path: r.ExPath},
		TerminateTimeout: 0,
	})
}
