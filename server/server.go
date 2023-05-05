package server

import (
	"github.com/0xJacky/Nginx-UI/server/analytic"
	"github.com/0xJacky/Nginx-UI/server/internal/cert"
	"github.com/0xJacky/Nginx-UI/server/internal/nginx"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/query"
	"github.com/0xJacky/Nginx-UI/server/router"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/go-co-op/gocron"
	"github.com/jpillora/overseer"
	"log"
	"mime"
	"net/http"
	"time"
)

func Program(state overseer.State) {
	// Hack: fix wrong Content Type of .js file on some OS platforms
	// See https://github.com/golang/go/issues/32350
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")

	log.Printf("Nginx config dir path: %s", nginx.GetConfPath())
	if "" != settings.ServerSettings.JwtSecret {
		db := model.Init()
		query.Init(db)
	}

	s := gocron.NewScheduler(time.UTC)
	job, err := s.Every(30).Minute().SingletonMode().Do(cert.AutoObtain)

	if err != nil {
		log.Fatalf("AutoCert Job: %v, Err: %v\n", job, err)
	}

	s.StartAsync()

	go analytic.RecordServerAnalytic()

	err = http.Serve(state.Listener, router.InitRouter())
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("[Nginx UI] server exiting")
}
