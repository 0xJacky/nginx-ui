package router

import (
	"encoding/base64"
	"github.com/0xJacky/Nginx-UI/frontend"
	"github.com/0xJacky/Nginx-UI/server/internal/logger"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"io/fs"
	"net/http"
	"path"
	"runtime"
	"strings"
)

func recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				errorAction := "panic"
				if action, ok := c.Get("maybe_error"); ok {
					errorActionMsg := cast.ToString(action)
					if errorActionMsg != "" {
						errorAction = errorActionMsg
					}
				}
				buf := make([]byte, 1024)
				runtime.Stack(buf, false)
				logger.Errorf("%s\n%s", err, buf)

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"message": err.(error).Error(),
					"error":   errorAction,
				})
			}
		}()

		c.Next()
	}
}

func authRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		abortWithAuthFailure := func() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "Authorization failed",
			})
		}

		token := c.GetHeader("Authorization")
		if token == "" {
			if token = c.GetHeader("X-Node-Secret"); token != "" && token == settings.ServerSettings.NodeSecret {
				c.Set("NodeSecret", token)
				c.Next()
				return
			} else {
				c.Set("ProxyNodeID", c.Query("x_node_id"))
				tokenBytes, _ := base64.StdEncoding.DecodeString(c.Query("token"))
				token = string(tokenBytes)
				if token == "" {
					abortWithAuthFailure()
					return
				}
			}
		}

		if model.CheckToken(token) < 1 {
			abortWithAuthFailure()
			return
		}

		if nodeID := c.GetHeader("X-Node-ID"); nodeID != "" {
			c.Set("ProxyNodeID", nodeID)
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
		defer func(file http.File) {
			err = file.Close()
			if err != nil {
				logger.Error("file not found", err)
			}
		}(file)
	}
	return err == nil
}

func mustFS(dir string) (serverFileSystem static.ServeFileSystem) {

	sub, err := fs.Sub(frontend.DistFS, path.Join("dist", dir))

	if err != nil {
		logger.Error(err)
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
