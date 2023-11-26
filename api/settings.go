package api

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
)

func GetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"server": settings.ServerSettings,
		"nginx":  settings.NginxSettings,
		"openai": settings.OpenAISettings,
	})
}

func SaveSettings(c *gin.Context) {
	var json struct {
		Server settings.Server `json:"server"`
		Nginx  settings.Nginx  `json:"nginx"`
		Openai settings.OpenAI `json:"openai"`
	}

	if !BindAndValid(c, &json) {
		return
	}

	settings.ServerSettings = json.Server
	settings.NginxSettings = json.Nginx
	settings.OpenAISettings = json.Openai

	settings.ReflectFrom()

	err := settings.Save()
	if err != nil {
		ErrHandler(c, err)
		return
	}

	GetSettings(c)
}

func GetCasdoorUri(c *gin.Context) {
	endpoint := settings.ServerSettings.CasdoorEndpoint
	clientId := settings.ServerSettings.CasdoorClientId
	redirectUri := settings.ServerSettings.CasdoorRedirectUri
	state := settings.ServerSettings.CasdoorApplication
	fmt.Println(redirectUri)
	if endpoint == "" || clientId == "" || redirectUri == "" || state == "" {
		c.JSON(http.StatusOK, gin.H{
			"uri": "",
		})
		return
	}
	encodedRedirectUri := url.QueryEscape(redirectUri)
	c.JSON(http.StatusOK, gin.H{
		"uri": fmt.Sprintf("%s/login/oauth/authorize?client_id=%s&response_type=code&redirect_uri=%s&state=%s&scope=read", endpoint, clientId, encodedRedirectUri, state),
	})
}
