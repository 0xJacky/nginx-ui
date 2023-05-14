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
		"openai":    settings.OpenAISettings,
		"git":       settings.GitSettings,
	})
}

func SaveSettings(c *gin.Context) {
	var json struct {
		Server   settings.Server   `json:"server"`
		NginxLog settings.NginxLog `json:"nginx_log"`
		Openai   settings.OpenAI   `json:"openai"`
	}

	if !BindAndValid(c, &json) {
		return
	}

	settings.ServerSettings = &json.Server
	settings.NginxLogSettings = &json.NginxLog
	settings.OpenAISettings = &json.Openai

	settings.ReflectFrom()

	err := settings.Save()
	if err != nil {
		ErrHandler(c, err)
		return
	}

	GetSettings(c)
}
