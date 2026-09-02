package settings

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/gin-gonic/gin"
)

// requireVerifiedTwoFactor gates endpoints that hand a stored secret back to
// the caller, so a node principal is refused outright.
func requireVerifiedTwoFactor(c *gin.Context, message string) bool {
	if _, ok := c.Get(nodeauth.GinPrincipalKey); ok {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"message": "Node secret authentication is not allowed for protected settings",
		})
		return false
	}

	return requireVerifiedSession(c, message)
}

// requireVerifiedTwoFactorOrProxy gates endpoints that only store a value. See
// middleware.VerifiedTwoFactorOrProxy for why a node principal is accepted.
func requireVerifiedTwoFactorOrProxy(c *gin.Context, message string) bool {
	return middleware.VerifiedTwoFactorOrProxy(c, message)
}

func requireVerifiedSession(c *gin.Context, message string) bool {
	return middleware.VerifiedSecureSession(c, message)
}

func GetProtectedSetting(c *gin.Context) {
	if !requireVerifiedTwoFactor(c, "Two-factor authentication is required to reveal protected settings") {
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
