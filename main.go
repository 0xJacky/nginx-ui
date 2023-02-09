package main

import (
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/server/analytic"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/pkg/cert"
	"github.com/0xJacky/Nginx-UI/server/pkg/nginx"
	"github.com/0xJacky/Nginx-UI/server/router"
	"github.com/0xJacky/Nginx-UI/server/service"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/gin-gonic/gin"
	"github.com/go-co-op/gocron"
	"github.com/jpillora/overseer"
	"github.com/jpillora/overseer/fetcher"
)

func main() {
	var confPath string
	flag.StringVar(&confPath, "config", "app.ini", "Specify the configuration file")
	flag.Parse()

	settings.Init(confPath)

	gin.SetMode(settings.ServerSettings.RunMode)

	r, err := service.GetRuntimeInfo()
	if err != nil {
		log.Fatalln(err)
	}
	overseer.Run(overseer.Config{
		Program:          prog,
		Address:          fmt.Sprintf(":%s", settings.ServerSettings.HttpPort),
		Fetcher:          &fetcher.File{Path: r.ExPath},
		TerminateTimeout: 0,
	})
}

func prog(state overseer.State) {
	// Hack: fix wrong Content Type of .js file on some OS platforms
	// See https://github.com/golang/go/issues/32350
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")

	log.Printf("Nginx config dir path: %s", nginx.GetConfPath())
	if "" != settings.ServerSettings.JwtSecret {
		model.Init()

		s := gocron.NewScheduler(time.UTC)
		job, err := s.Every(1).Hour().SingletonMode().Do(cert.AutoCert)

		if err != nil {
			log.Fatalf("AutoCert Job: %v, Err: %v\n", job, err)
		}

		s.StartAsync()

		go analytic.RecordServerAnalytic()
	}

	db, err := model.Init()
	if err != nil {
		log.Fatalln(err)
	}
	srv := model.NewService(db)
	log.Println("server starting on", state.Listener.Addr().String())
	h := router.Handler{
		Srv: srv,
	}
	err = http.Serve(state.Listener, h.InitRouter())
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("[Nginx UI] server exiting")
}
