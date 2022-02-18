package router

import (
	"bufio"
	"encoding/base64"
	"github.com/0xJacky/Nginx-UI/frontend"
	api2 "github.com/0xJacky/Nginx-UI/server/api"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
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

type serverFileSystemType struct {
	http.FileSystem
}

func (f serverFileSystemType) Exists(prefix string, filePath string) bool {
	_, err := f.Open(path.Join(prefix, filePath))
	return err == nil
}

func mustFS(dir string) (serverFileSystem static.ServeFileSystem) {

	sub, err := fs.Sub(frontend.DistFS, path.Join("dist", dir))

	if err != nil {
		log.Println(err)
	}

	serverFileSystem = serverFileSystemType{
		http.FS(sub),
	}

	return
}

func InitRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())

	r.Use(gin.Recovery())

	r.Use(static.Serve("/", mustFS("")))

	r.NoRoute(func(c *gin.Context) {
		accept := c.Request.Header.Get("Accept")
		if strings.Contains(accept, "text/html") {
			file, _ := mustFS("").Open("index.html")
			stat, _ := file.Stat()
			c.DataFromReader(http.StatusOK, stat.Size(), "text/html",
				bufio.NewReader(file), nil)
		}
	})

	g := r.Group("/api")
	{
		g.GET("install", api2.InstallLockCheck)
		g.POST("install", api2.InstallNginxUI)

		g.POST("/login", api2.Login)
		g.DELETE("/logout", api2.Logout)

		g := g.Group("/", authRequired())
		{
			g.GET("/analytic", api2.Analytic)

			g.GET("/users", api2.GetUsers)
			g.GET("/user/:id", api2.GetUser)
			g.POST("/user", api2.AddUser)
			g.POST("/user/:id", api2.EditUser)
			g.DELETE("/user/:id", api2.DeleteUser)

			g.GET("domains", api2.GetDomains)
			g.GET("domain/:name", api2.GetDomain)
			g.POST("domain/:name", api2.EditDomain)
			g.POST("domain/:name/enable", api2.EnableDomain)
			g.POST("domain/:name/disable", api2.DisableDomain)
			g.DELETE("domain/:name", api2.DeleteDomain)

			g.GET("configs", api2.GetConfigs)
			g.GET("config/:name", api2.GetConfig)
			g.POST("config", api2.AddConfig)
			g.POST("config/:name", api2.EditConfig)

			g.GET("backups", api2.GetFileBackupList)
			g.GET("backup/:id", api2.GetFileBackup)

			g.GET("template/:name", api2.GetTemplate)

			g.GET("cert/issue/:domain", api2.IssueCert)
			g.GET("cert/:domain/info", api2.CertInfo)

			// 添加域名到自动续期列表
			g.POST("cert/:domain", api2.AddDomainToAutoCert)
			// 从自动续期列表中删除域名
			g.DELETE("cert/:domain", api2.RemoveDomainFromAutoCert)
		}
	}

	return r
}
