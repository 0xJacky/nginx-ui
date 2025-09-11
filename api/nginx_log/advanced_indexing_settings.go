package nginx_log

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// EnableAdvancedIndexing enables advanced indexing for nginx logs
func EnableAdvancedIndexing(c *gin.Context) {
	settings.NginxLogSettings.AdvancedIndexingEnabled = true

	err := settings.Save()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Start the nginx_log services
	nginx_log.InitializeModernServices(kernel.Context)

	c.JSON(http.StatusOK, gin.H{
		"message": "Advanced indexing enabled successfully",
	})
}

// DisableAdvancedIndexing disables advanced indexing for nginx logs
func DisableAdvancedIndexing(c *gin.Context) {
	settings.NginxLogSettings.AdvancedIndexingEnabled = false

	err := settings.Save()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Stop the nginx_log services
	nginx_log.StopModernServices()

	c.JSON(http.StatusOK, gin.H{
		"message": "Advanced indexing disabled successfully",
	})
}

// GetAdvancedIndexingStatus returns the current status of advanced indexing
func GetAdvancedIndexingStatus(c *gin.Context) {
	enabled := settings.NginxLogSettings.AdvancedIndexingEnabled

	c.JSON(http.StatusOK, gin.H{
		"enabled": enabled,
	})
}