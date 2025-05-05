package upgrader

import (
	"os"

	"code.pfad.fr/risefront"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

type Control struct {
	DryRun  bool   `json:"dry_run"`
	Channel string `json:"channel"`
}

// BinaryUpgrade Upgrade the binary
func BinaryUpgrade(ws *websocket.Conn, control *Control) {
	_ = ws.WriteJSON(CoreUpgradeResp{
		Status:  UpgradeStatusInfo,
		Message: "Initialing core upgrader",
	})

	u, err := NewUpgrader(control.Channel)
	if err != nil {
		_ = ws.WriteJSON(CoreUpgradeResp{
			Status:  UpgradeStatusError,
			Message: "Initial core upgrader error",
		})
		_ = ws.WriteJSON(CoreUpgradeResp{
			Status:  UpgradeStatusError,
			Message: err.Error(),
		})
		logger.Error(err)
		return
	}
	_ = ws.WriteJSON(CoreUpgradeResp{
		Status:  UpgradeStatusInfo,
		Message: "Downloading latest release",
	})
	progressChan := make(chan float64)
	defer close(progressChan)
	go func() {
		for progress := range progressChan {
			_ = ws.WriteJSON(CoreUpgradeResp{
				Status:   UpgradeStatusProgress,
				Progress: progress,
			})
		}
	}()

	tarName, err := u.DownloadLatestRelease(progressChan)
	if err != nil {
		_ = ws.WriteJSON(CoreUpgradeResp{
			Status:  UpgradeStatusError,
			Message: "Download latest release error",
		})
		_ = ws.WriteJSON(CoreUpgradeResp{
			Status:  UpgradeStatusError,
			Message: err.Error(),
		})
		logger.Error(err)
		return
	}

	defer func() {
		_ = os.Remove(tarName)
		_ = os.Remove(tarName + ".digest")
	}()
	_ = ws.WriteJSON(CoreUpgradeResp{
		Status:  UpgradeStatusInfo,
		Message: "Performing core upgrade",
	})
	// dry run
	if control.DryRun || settings.NodeSettings.Demo {
		risefront.Restart()
		return
	}

	// bye, will restart nginx-ui in performCoreUpgrade
	err = u.PerformCoreUpgrade(tarName)
	if err != nil {
		_ = ws.WriteJSON(CoreUpgradeResp{
			Status:  UpgradeStatusError,
			Message: "Perform core upgrade error",
		})
		_ = ws.WriteJSON(CoreUpgradeResp{
			Status:  UpgradeStatusError,
			Message: err.Error(),
		})
		logger.Error(err)
		return
	}
}
