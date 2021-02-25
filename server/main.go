package main

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/router"
	"github.com/0xJacky/Nginx-UI/settings"
	"log"
)

func main() {
	settings.Init()

	r := router.InitRouter()

	model.Init()

	err := r.Run(":" + settings.ServerSettings.HttpPort)

	if err != nil {
		log.Fatal(err)
	}
}
