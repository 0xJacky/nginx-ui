package router

import (
	"encoding/base64"
	"github.com/0xJacky/Nginx-UI/app"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
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
				buf := make([]byte, 1024)
				runtime.Stack(buf, false)
				logger.Errorf("%s\n%s", err, buf)

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"message": err.(error).Error(),
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

		u, ok := user.GetTokenUser(token)
		if !ok {
			abortWithAuthFailure()
			return
		}

		c.Set("user", u)

		if nodeID := c.GetHeader("X-Node-ID"); nodeID != "" {
			c.Set("ProxyNodeID", nodeID)
		}

		c.Next()
	}
}

func required2FA() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := c.Get("user")
		if !ok {
			c.Next()
			return
		}
		cUser := u.(*model.Auth)
		if !cUser.EnabledOTP() {
			c.Next()
			return
		}
		ssid := c.GetHeader("X-Secure-Session-ID")
		if ssid == "" {
			ssid = c.Query("X-Secure-Session-ID")
		}
		if ssid == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "Secure Session ID is empty",
			})
			return
		}

		if user.VerifySecureSessionID(ssid, cUser.ID) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"message": "Secure Session ID is invalid",
		})
		return
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

	sub, err := fs.Sub(app.DistFS, path.Join("dist", dir))

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
