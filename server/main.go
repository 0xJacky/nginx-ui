package main

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/router"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/0xJacky/Nginx-UI/tool"
	"log"
)

func main() {
	settings.Init()

	r := router.InitRouter()

	model.Init()

	log.Printf("nginx config dir path: %s", tool.GetNginxConfPath(""))

	err := r.Run(":" + settings.ServerSettings.HttpPort)

	if err != nil {
		log.Fatal(err)
	}
}
