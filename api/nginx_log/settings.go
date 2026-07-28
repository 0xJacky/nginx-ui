package nginx_log

import (
	"net/http"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// EnableAdvancedIndexing enables advanced indexing for nginx logs
func EnableAdvancedIndexing(c *gin.Context) {
	err := settings.Update(func() {
		settings.NginxLogSettings.IndexingEnabled = true
	})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Start the nginx_log services
	nginx_log.InitializeServices(kernel.Context)

	// Migrate fallback cache entries to LogFileManager
	nginx_log.MigrateFallbackCache()

	c.JSON(http.StatusOK, gin.H{
		"message": "Advanced indexing enabled successfully",
	})
}

// DisableAdvancedIndexing disables advanced indexing for nginx logs
func DisableAdvancedIndexing(c *gin.Context) {
	err := settings.Update(func() {
		settings.NginxLogSettings.IndexingEnabled = false
	})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Stop the nginx_log services
	nginx_log.StopServices()

	c.JSON(http.StatusOK, gin.H{
		"message": "Advanced indexing disabled successfully",
	})
}

// GetAdvancedIndexingStatus returns the current status of advanced indexing
func GetAdvancedIndexingStatus(c *gin.Context) {
	enabled := settings.NginxLogSettings.IndexingEnabled

	c.JSON(http.StatusOK, gin.H{
		"enabled": enabled,
	})
}

// GetDefaultLogDir returns the directory nginx writes its default access log
// to. The site editor uses it to propose a per-site access_log path: a log
// placed next to the default one is inside the log directory whitelist, so it
// can be read and indexed without any further configuration.
func GetDefaultLogDir(c *gin.Context) {
	dir := ""
	if accessLogPath := nginx.GetAccessLogPath(); accessLogPath != "" {
		dir = filepath.Dir(accessLogPath)
	} else if prefix := nginx.GetPrefix(); prefix != "" {
		// nginx may not be running or may declare no access_log at all; the
		// logs directory under the nginx prefix is whitelisted too.
		dir = filepath.Join(prefix, "logs")
	}

	c.JSON(http.StatusOK, gin.H{
		"access_log_dir": dir,
	})
}
