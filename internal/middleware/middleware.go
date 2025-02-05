package middleware

import (
	"encoding/base64"
	"net/http"
	"path"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy/logger"
)

// getToken from header, cookie or query
func getToken(c *gin.Context) (token string) {
	if token = c.GetHeader("Authorization"); token != "" {
		return
	}

	if token, _ = c.Cookie("token"); token != "" {
		return token
	}

	if token = c.Query("token"); token != "" {
		tokenBytes, _ := base64.StdEncoding.DecodeString(token)
		return string(tokenBytes)
	}

	return ""
}

// getXNodeID from header or query
func getXNodeID(c *gin.Context) (xNodeID string) {
	if xNodeID = c.GetHeader("X-Node-ID"); xNodeID != "" {
		return xNodeID
	}

	return c.Query("x_node_id")
}

// AuthRequired is a middleware that checks if the user is authenticated
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		abortWithAuthFailure := func() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "Authorization failed",
			})
		}

		xNodeID := getXNodeID(c)
		if xNodeID != "" {
			c.Set("ProxyNodeID", xNodeID)
		}

		token := getToken(c)
		if token == "" {
			if token = c.GetHeader("X-Node-Secret"); token != "" && token == settings.NodeSettings.Secret {
				c.Set("Secret", token)
				c.Next()
				return
			} else {
				abortWithAuthFailure()
				return
			}
		}

		u, ok := user.GetTokenUser(token)
		if !ok {
			abortWithAuthFailure()
			return
		}

		c.Set("user", u)
		c.Next()
	}
}

type ServerFileSystemType struct {
	http.FileSystem
}

func (f ServerFileSystemType) Exists(prefix string, _path string) bool {
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

// CacheJs is a middleware that send header to client to cache js file
func CacheJs() gin.HandlerFunc {
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
