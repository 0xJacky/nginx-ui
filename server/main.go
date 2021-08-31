package main

import (
    "flag"
    "github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/router"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/0xJacky/Nginx-UI/server/tool"
	"log"
)

func main() {
    var dbPath string
    var confPath string
    flag.StringVar(&confPath, "c", "app.ini", "Specify the conf path to load")
    flag.StringVar(&dbPath, "d", "database.db", "Specify the database path to load")
    flag.Parse()

	settings.Init(confPath)

	r := router.InitRouter()

	model.Init(dbPath)

	log.Printf("nginx config dir path: %s", tool.GetNginxConfPath(""))

	err := r.Run(":" + settings.ServerSettings.HttpPort)

	if err != nil {
		log.Fatal(err)
	}
}
