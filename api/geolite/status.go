package geolite

import (
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/geolite"
	"github.com/gin-gonic/gin"
)

type StatusResp struct {
	Exists       bool   `json:"exists"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	LastModified string `json:"last_modified"`
}

func GetStatus(c *gin.Context) {
	// Goes through CurrentAvailability rather than stat'ing the path directly,
	// so a demo instance can report the database as present without shipping it.
	availability := geolite.CurrentAvailability()

	resp := StatusResp{
		Exists: availability.Exists,
		Path:   availability.Path,
		Size:   availability.Size,
	}
	if !availability.LastModified.IsZero() {
		resp.LastModified = availability.LastModified.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, resp)
}
