package settings

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	cSettings "github.com/uozi-tech/cosy/settings"
)

var protectedSettingRevealAllowlist = map[string]func() string{
	"app.jwt_secret": func() string {
		return cSettings.AppSettings.JwtSecret
	},
	"node.secret": func() string {
		return settings.NodeSettings.Secret
	},
	"openai.token": func() string {
		return settings.OpenAISettings.Token
	},
}

func GetProtectedSetting(c *gin.Context) {
	if _, ok := c.Get("Secret"); ok {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"message": "Node secret authentication is not allowed for protected settings",
		})
		return
	}

	path := c.Query("path")
	getter, ok := protectedSettingRevealAllowlist[path]
	if !ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message": "Protected setting path is invalid",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"value": getter(),
	})
}
