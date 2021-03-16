package router

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/gin-gonic/gin"
	"net/http"
    "github.com/gin-contrib/cors"
)

func InitRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())

	r.Use(gin.Recovery())

    r.Use(cors.Default())

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello World",
		})
	})

	endpoint := r.Group("/")
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
	}

	return r
}
