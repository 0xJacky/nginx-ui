package upgrader

import (
	"github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

func DockerUpgrade(ws *websocket.Conn, control *Control) {
	err := docker.UpgradeStepOne(control.Channel)
	if err != nil {
		_ = ws.WriteJSON(CoreUpgradeResp{
			Status:  UpgradeStatusError,
			Message: err.Error(),
		})
		logger.Error(err)
		return
	}
}
