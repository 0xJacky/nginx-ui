package api

import (
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"server":    settings.ServerSettings,
		"nginx_log": settings.NginxLogSettings,
	})
}

func SaveSettings(c *gin.Context) {
	var json struct {
		Server   settings.Server   `json:"server"`
		NginxLog settings.NginxLog `json:"nginx_log"`
	}

	if !BindAndValid(c, &json) {
		return
	}

	settings.Conf.Section("server").Key("Email").SetValue(json.Server.Email)
	settings.Conf.Section("server").Key("HTTPChallengePort").SetValue(json.Server.HTTPChallengePort)
	settings.Conf.Section("nginx_log").Key("AccessLogPath").SetValue(json.NginxLog.AccessLogPath)
	settings.Conf.Section("nginx_log").Key("ErrorLogPath").SetValue(json.NginxLog.ErrorLogPath)

	err := settings.Save()
	if err != nil {
		ErrHandler(c, err)
		return
	}

	GetSettings(c)
}
