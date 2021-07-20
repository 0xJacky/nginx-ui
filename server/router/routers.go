package router

import (
    "encoding/base64"
    "github.com/0xJacky/Nginx-UI/api"
    "github.com/0xJacky/Nginx-UI/model"
    "github.com/gin-gonic/gin"
    "net/http"
)

func authRequired() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        if token == "" {
            tmp, _ := base64.StdEncoding.DecodeString(c.Query("token"))
            token = string(tmp)
            if token == "" {
                c.JSON(http.StatusForbidden, gin.H{
                    "message": "auth fail",
                })
                c.Abort()
                return
            }
        }

        n := model.CheckToken(token)

        if n < 1 {
            c.JSON(http.StatusForbidden, gin.H{
                "message": "auth fail",
            })
            c.Abort()
            return
        }
        c.Next()
    }
}

func InitRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())

	r.Use(gin.Recovery())

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello World",
		})
	})

	r.POST("/login", api.Login)
    r.DELETE("/logout", api.Logout)

	endpoint := r.Group("/", authRequired())
	{
		endpoint.GET("domains", api.GetDomains)
		endpoint.GET("domain/:name", api.GetDomain)
		endpoint.POST("domain/:name", api.EditDomain)
		endpoint.POST("domain/:name/enable", api.EnableDomain)
		endpoint.POST("domain/:name/disable", api.DisableDomain)
		endpoint.DELETE("domain/:name", api.DeleteDomain)

		endpoint.GET("configs", api.GetConfigs)
		endpoint.GET("config/:name", api.GetConfig)
		endpoint.POST("config", api.AddConfig)
		endpoint.POST("config/:name", api.EditConfig)

		endpoint.GET("backups", api.GetFileBackupList)
		endpoint.GET("backup/:id", api.GetFileBackup)

        endpoint.GET("template/:name", api.GetTemplate)

        endpoint.GET("analytic", api.Analytic)

		endpoint.GET("cert/issue/:domain", api.IssueCert)
        endpoint.GET("cert/:domain/info", api.CertInfo)
	}

	return r
}
