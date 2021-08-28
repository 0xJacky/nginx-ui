package main

import (
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/router"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/0xJacky/Nginx-UI/server/tool"
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
