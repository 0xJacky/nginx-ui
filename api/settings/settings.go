package settings

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/cron"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	cSettings "github.com/uozi-tech/cosy/settings"
	"net/http"
)

func GetServerName(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"name": settings.NodeSettings.Name,
	})
}

func GetSettings(c *gin.Context) {
	settings.NginxSettings.AccessLogPath = nginx.GetAccessLogPath()
	settings.NginxSettings.ErrorLogPath = nginx.GetErrorLogPath()
	c.JSON(http.StatusOK, gin.H{
		"server":    cSettings.ServerSettings,
		"nginx":     settings.NginxSettings,
		"openai":    settings.OpenAISettings,
		"logrotate": settings.LogrotateSettings,
		"auth":      settings.AuthSettings,
	})
}

func SaveSettings(c *gin.Context) {
	var json struct {
		Server    cSettings.Server   `json:"server"`
		Nginx     settings.Nginx     `json:"nginx"`
		Openai    settings.OpenAI    `json:"openai"`
		Logrotate settings.Logrotate `json:"logrotate"`
		Auth      settings.Auth      `json:"auth"`
	}

	if !api.BindAndValid(c, &json) {
		return
	}

	if settings.LogrotateSettings.Enabled != json.Logrotate.Enabled ||
		settings.LogrotateSettings.Interval != json.Logrotate.Interval {
		go cron.RestartLogrotate()
	}

	cSettings.ProtectedFill(cSettings.ServerSettings, &json.Server)
	cSettings.ProtectedFill(settings.NginxSettings, &json.Nginx)
	cSettings.ProtectedFill(settings.OpenAISettings, &json.Openai)
	cSettings.ProtectedFill(settings.LogrotateSettings, &json.Logrotate)
	cSettings.ProtectedFill(settings.AuthSettings, &json.Auth)

	err := settings.Save()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	GetSettings(c)
}
