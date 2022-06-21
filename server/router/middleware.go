package router

import (
	"encoding/base64"
	"github.com/0xJacky/Nginx-UI/frontend"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
)

func recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Println(err)
			}
		}()

		c.Next()
	}
}

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

func (f serverFileSystemType) Exists(prefix string, _path string) bool {
	file, err := f.Open(path.Join(prefix, _path))
	if file != nil {
		defer file.Close()
	}
	return err == nil
}

func mustFS(dir string) (serverFileSystem static.ServeFileSystem) {

	sub, err := fs.Sub(frontend.DistFS, path.Join("dist", dir))

	if err != nil {
		log.Println(err)
		return
	}

	serverFileSystem = serverFileSystemType{
		http.FS(sub),
	}

	return
}

func cacheJs() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.Contains(c.Request.URL.String(), "js") {
			c.Header("Cache-Control", "max-age: 1296000")
			if c.Request.Header.Get("If-Modified-Since") == settings.LastModified {
				c.AbortWithStatus(http.StatusNotModified)
			}
			c.Header("Last-Modified", settings.LastModified)
		}
	}
}
