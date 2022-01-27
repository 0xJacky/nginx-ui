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
