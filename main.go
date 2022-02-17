package main

import (
	"flag"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/router"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/0xJacky/Nginx-UI/tool"
	"log"
)

func main() {
	var dataDir string
	flag.StringVar(&dataDir, "d", ".", "Specify the data dir")
	flag.Parse()

	settings.Init(dataDir)
	model.Init()

	r := router.InitRouter()

	log.Printf("nginx config dir path: %s", tool.GetNginxConfPath(""))

	go tool.AutoCert()

	err := r.Run(":" + settings.ServerSettings.HttpPort)

	if err != nil {
		log.Fatal(err)
	}

}
