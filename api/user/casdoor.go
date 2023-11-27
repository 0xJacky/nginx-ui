package user

import (
    "fmt"
    "github.com/0xJacky/Nginx-UI/settings"
    "github.com/gin-gonic/gin"
    "net/http"
    "net/url"
)

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
