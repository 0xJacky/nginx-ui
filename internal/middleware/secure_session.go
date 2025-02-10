package middleware

import (
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

func RequireSecureSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := c.Get("user")
		if !ok {
			c.Next()
			return
		}
		cUser := u.(*model.User)
		if !cUser.EnabledOTP() {
			c.Next()
			return
		}
		ssid := c.GetHeader("X-Secure-Session-ID")
		if ssid == "" {
			ssid = c.Query("X-Secure-Session-ID")
		}
		if ssid == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Secure Session ID is empty",
			})
			return
		}

		if user.VerifySecureSessionID(ssid, cUser.ID) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"message": "Secure Session ID is invalid",
		})
		return
	}
}
