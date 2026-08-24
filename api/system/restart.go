package system

import (
	"net/http"
	"os/exec"
	"runtime"
	"strings"

	"code.pfad.fr/risefront"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy/logger"
)

func Restart(c *gin.Context) {
	restartCmd := strings.TrimSpace(settings.NodeSettings.RestartCmd)
	if restartCmd != "" {
		name := "/bin/sh"
		args := []string{"-c", restartCmd}
		if runtime.GOOS == "windows" {
			name = "cmd"
			args = []string{"/c", restartCmd}
		}

		cmd := exec.Command(name, args...)
		if err := cmd.Start(); err != nil {
			logger.Errorf("system restart command failed to start: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "failed to start restart command",
				"error":   err.Error(),
			})
			return
		}

		logger.Infof("system restart command started: %s", restartCmd)
		c.JSON(http.StatusOK, gin.H{
			"message": "restart command started",
		})
		return
	}

	risefront.Restart()

	c.JSON(http.StatusOK, gin.H{
		"message": "restarting...",
	})
}
