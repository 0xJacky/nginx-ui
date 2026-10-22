package upgrader

import (
	"github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/uozi-tech/cosy/logger"
)

// DockerUpgrade Upgrade the Docker container
func DockerUpgrade(ws *helper.SafeWebSocketWriter, control *Control) {
	progressChan, stopProgress := startUpgradeProgress(ws, "Pulling Docker image...")
	defer stopProgress()

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
