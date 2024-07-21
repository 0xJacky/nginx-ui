package router

import (
    "github.com/0xJacky/Nginx-UI/api/analytic"
    "github.com/0xJacky/Nginx-UI/api/certificate"
    "github.com/0xJacky/Nginx-UI/api/cluster"
    "github.com/0xJacky/Nginx-UI/api/config"
    "github.com/0xJacky/Nginx-UI/api/nginx"
    "github.com/0xJacky/Nginx-UI/api/notification"
    "github.com/0xJacky/Nginx-UI/api/openai"
    "github.com/0xJacky/Nginx-UI/api/settings"
    "github.com/0xJacky/Nginx-UI/api/sites"
    "github.com/0xJacky/Nginx-UI/api/streams"
    "github.com/0xJacky/Nginx-UI/api/system"
    "github.com/0xJacky/Nginx-UI/api/template"
    "github.com/0xJacky/Nginx-UI/api/terminal"
    "github.com/0xJacky/Nginx-UI/api/upstream"
    "github.com/0xJacky/Nginx-UI/api/user"
    "github.com/gin-contrib/static"
    "github.com/gin-gonic/gin"
    "net/http"
)

func InitRouter() *gin.Engine {
    r := gin.New()
    r.Use(gin.Logger())
    r.Use(recovery())
    r.Use(cacheJs())
    r.Use(ipWhiteList())

    //r.Use(OperationSync())

    r.Use(static.Serve("/", mustFS("")))

    r.NoRoute(func(c *gin.Context) {
        c.JSON(http.StatusNotFound, gin.H{
            "message": "not found",
        })
    })

    root := r.Group("/api")
    {
        system.InitPublicRouter(root)
        user.InitAuthRouter(root)

        // Authorization required not websocket request
        g := root.Group("/", authRequired(), proxy())
        {
            analytic.InitRouter(g)
            user.InitManageUserRouter(g)
            nginx.InitRouter(g)
            sites.InitRouter(g)
            streams.InitRouter(g)
            config.InitRouter(g)
            template.InitRouter(g)
            certificate.InitCertificateRouter(g)
            certificate.InitDNSCredentialRouter(g)
            certificate.InitAcmeUserRouter(g)
            system.InitPrivateRouter(g)
            settings.InitRouter(g)
            openai.InitRouter(g)
            cluster.InitRouter(g)
            notification.InitRouter(g)
        }

        // Authorization required and websocket request
        w := root.Group("/", authRequired(), proxyWs())
        {
            analytic.InitWebSocketRouter(w)
            certificate.InitCertificateWebSocketRouter(w)
            terminal.InitRouter(w)
            nginx.InitNginxLogRouter(w)
            upstream.InitRouter(w)
            system.InitWebSocketRouter(w)
        }
    }

    return r
}
