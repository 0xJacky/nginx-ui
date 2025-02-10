//go:build !unembed

package middleware

import (
	"io/fs"
	"net/http"
	"path"

	"github.com/0xJacky/Nginx-UI/app"
	"github.com/gin-contrib/static"
	"github.com/uozi-tech/cosy/logger"
)

func MustFs(dir string) (serverFileSystem static.ServeFileSystem) {

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
