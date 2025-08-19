//go:build !unembed

package middleware

import (
	"path"

	"github.com/0xJacky/Nginx-UI/app"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/spf13/afero"
	"github.com/uozi-tech/cosy/logger"
)

func mustFs(dir string) (serverFileSystem static.ServeFileSystem) {
	fs, err := app.GetDistFS()
	if err != nil {
		logger.Error(err)
		return
	}
	
	// Create a sub filesystem for the dist directory
	subFS := afero.NewBasePathFs(fs, path.Join("dist", dir))
	httpSubFS := afero.NewHttpFs(subFS)
	
	serverFileSystem = ServerFileSystemType{
		httpSubFS,
	}
	return
}

func ServeStatic() gin.HandlerFunc {
	return static.Serve("/", mustFs(""))
}