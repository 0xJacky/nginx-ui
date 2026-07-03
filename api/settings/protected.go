package settings

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func GetProtectedSetting(c *gin.Context) {
	if _, ok := c.Get("Secret"); ok {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"message": "Node secret authentication is not allowed for protected settings",
		})
		return
	}

	if verified, _ := c.Get(middleware.SecureSessionVerifiedKey); verified != true {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"message": "Two-factor authentication is required to reveal protected settings",
		})
		return
	}

	path := c.Query("path")
	value, ok := getProtectedSettingValue(path)
	if !ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message": "Protected setting path is invalid",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"value": value,
	})
}
