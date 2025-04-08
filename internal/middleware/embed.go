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

func ServeStatic() gin.HandlerFunc {
	return static.Serve("/", mustFs(""))
}
