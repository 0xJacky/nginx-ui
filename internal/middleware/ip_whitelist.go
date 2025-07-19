package middleware

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

func IPWhiteList() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		if len(settings.AuthSettings.IPWhiteList) == 0 || clientIP == "127.0.0.1" || clientIP == "::1" {
			c.Next()
			return
		}

		if !lo.Contains(settings.AuthSettings.IPWhiteList, clientIP) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}
