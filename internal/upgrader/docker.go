package upgrader

import (
	"github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

// DockerUpgrade Upgrade the Docker container
func DockerUpgrade(ws *websocket.Conn, control *Control) {
	progressChan := make(chan float64)

	// Start a goroutine to listen for progress updates and send them via WebSocket
	go func() {
		for progress := range progressChan {
			err := ws.WriteJSON(CoreUpgradeResp{
				Status:   UpgradeStatusProgress,
				Progress: progress,
				Message:  "Pulling Docker image...",
			})
			if err != nil {
				logger.Error("Failed to send progress update:", err)
				return
			}
		}
	}()
	defer close(progressChan)

	if !control.DryRun {
		err := docker.UpgradeStepOne(control.Channel, progressChan)
		if err != nil {
			_ = ws.WriteJSON(CoreUpgradeResp{
				Status:  UpgradeStatusError,
				Message: err.Error(),
			})
			logger.Error(err)
			return
		}
	}

	// Send completion message
	_ = ws.WriteJSON(CoreUpgradeResp{
		Status:   UpgradeStatusInfo,
		Progress: 100,
		Message:  "Docker image pull completed, upgrading...",
	})
}
