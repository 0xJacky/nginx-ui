package public

import (
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"net/http"
)

// GetICPSettings returns the handful of node facts the SPA needs before it has
// a session. `demo` is included so the frontend can decide what to render
// without waiting for the authenticated GET /api/settings, which only resolves
// after a view has already mounted.
func GetICPSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"icp_number":             settings.NodeSettings.ICPNumber,
		"public_security_number": settings.NodeSettings.PublicSecurityNumber,
		"demo":                   settings.NodeSettings.Demo,
	})
}
