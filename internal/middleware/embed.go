//go:build !unembed

package middleware

import (
	"io/fs"
	"net/http"
	"path"

	"github.com/0xJacky/Nginx-UI/app"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy/logger"
)

func mustFs(dir string) (serverFileSystem static.ServeFileSystem) {
	sub, err := fs.Sub(app.DistFS, path.Join("dist", dir))
	if err != nil {
		logger.Error(err)
		return
	}
	serverFileSystem = ServerFileSystemType{
		http.FS(sub),
	}
	return
}

func ServeStatic() []gin.HandlerFunc {
	const urlPrefix = "/"
	fs := mustFs(urlPrefix)
	fileserver := http.FileServer(fs)
	if urlPrefix != "" {
		fileserver = http.StripPrefix(urlPrefix, fileserver)
	}
	return []gin.HandlerFunc{
		func(c *gin.Context) {
			if fs.Exists(urlPrefix, c.Request.URL.Path) {
				c.Next()
			}
		},
		IPWhiteList(),
		func(c *gin.Context) {
			fileserver.ServeHTTP(c.Writer, c.Request)
			c.Abort()
		},
	}
}
