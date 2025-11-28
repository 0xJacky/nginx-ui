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

func mustFs(dir string) static.ServeFileSystem {
	fsys, err := app.GetDistFS()
	if err != nil {
		logger.Error(err)
		return nil
	}

	distPath := path.Join("dist", dir)
	distFS, err := fs.Sub(fsys, distPath)
	if err != nil {
		logger.Error(err)
		return nil
	}

	return ServerFileSystemType{
		http.FS(distFS),
	}
}

func ServeStatic() gin.HandlerFunc {
	return static.Serve("/", mustFs(""))
}
