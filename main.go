package main

import (
    "flag"
    "fmt"
    "github.com/0xJacky/Nginx-UI/logger"
    "github.com/0xJacky/Nginx-UI/server"
    "github.com/0xJacky/Nginx-UI/server/service"
    "github.com/0xJacky/Nginx-UI/server/settings"
    "github.com/gin-gonic/gin"
    "github.com/jpillora/overseer"
    "github.com/jpillora/overseer/fetcher"
)

func main() {
    var confPath string
    flag.StringVar(&confPath, "config", "app.ini", "Specify the configuration file")
    flag.Parse()

    settings.Init(confPath)

    logger.Init(settings.ServerSettings.RunMode)

    gin.SetMode(settings.ServerSettings.RunMode)

    defer logger.Sync()

    r, err := service.GetRuntimeInfo()

    if err != nil {
        logger.Fatal(err)
    }

    overseer.Run(overseer.Config{
        Program:          server.Program,
        Address:          fmt.Sprintf(":%s", settings.ServerSettings.HttpPort),
        Fetcher:          &fetcher.File{Path: r.ExPath},
        TerminateTimeout: 0,
    })
}
