package system

import (
	"net/http"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/upgrader"
	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

func GetRelease(c *gin.Context) {
	data, err := upgrader.GetRelease(c.Query("channel"))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	runtimeInfo, err := upgrader.GetRuntimeInfo()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	type resp struct {
		upgrader.TRelease
		upgrader.RuntimeInfo
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

	var control struct {
		DryRun  bool   `json:"dry_run"`
		Channel string `json:"channel"`
	}

	err = ws.ReadJSON(&control)

	if err != nil {
		logger.Error(err)
		return
	}

	_ = ws.WriteJSON(CoreUpgradeResp{
		Status:  UpgradeStatusInfo,
		Message: "Initialing core upgrader",
	})

	u, err := upgrader.NewUpgrader(control.Channel)

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
