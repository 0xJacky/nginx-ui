package public

import (
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetICPSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"icp_number":             settings.NodeSettings.ICPNumber,
		"public_security_number": settings.NodeSettings.PublicSecurityNumber,
	})
}
