package system

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/upgrader"
	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

func GetRelease(c *gin.Context) {
	data, err := version.GetRelease(c.Query("channel"))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	runtimeInfo, err := version.GetRuntimeInfo()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	type resp struct {
		version.TRelease
		version.RuntimeInfo
	}
	c.JSON(http.StatusOK, resp{
		data, runtimeInfo,
	})
}

func GetCurrentVersion(c *gin.Context) {
	c.JSON(http.StatusOK, version.GetVersionInfo())
}

const (
	UpgradeStatusInfo     = "info"
	UpgradeStatusError    = "error"
	UpgradeStatusProgress = "progress"
)

type CoreUpgradeResp struct {
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Message  string  `json:"message"`
}

func PerformCoreUpgrade(c *gin.Context) {
	var upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	// upgrade http to websocket
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err)
		return
	}
	defer ws.Close()

	var control upgrader.Control

	err = ws.ReadJSON(&control)

	if err != nil {
		logger.Error(err)
		return
	}
	if helper.InNginxUIOfficialDocker() && helper.DockerSocketExists() {
		upgrader.DockerUpgrade(ws, &control)
	} else {
		upgrader.BinaryUpgrade(ws, &control)
	}
}
