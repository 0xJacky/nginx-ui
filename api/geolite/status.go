package geolite

import (
	"net/http"
	"os"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/geolite"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

type StatusResp struct {
	Exists       bool   `json:"exists"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	LastModified string `json:"last_modified"`
}

func GetStatus(c *gin.Context) {
	dbPath := geolite.GetDBPath()
	resp := StatusResp{
		Exists: geolite.DBExists(),
		Path:   dbPath,
	}

	if resp.Exists {
		fileInfo, err := os.Stat(dbPath)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
		resp.Size = fileInfo.Size()
		resp.LastModified = fileInfo.ModTime().Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, resp)
}
