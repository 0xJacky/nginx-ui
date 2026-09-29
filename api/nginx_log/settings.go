package nginx_log

import (
	"net/http"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
)

// GetLegacyIndexingStatus reports whether advanced indexing was enabled before
// log analytics moved into a plugin. The value is read only: it tells the UI to
// offer the plugin until the handoff to it has happened.
func GetLegacyIndexingStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"enabled": settings.NginxLogSettings.IndexingEnabled,
	})
}

// GetDefaultLogDir returns the directory nginx writes its default access log
// to. The site editor uses it to propose a per-site access_log path: a log
// placed next to the default one is inside the log directory whitelist, so it
// can be read without any further configuration.
//
// It also reports the default access and error log files themselves when they
// are readable through the log whitelist, for a site that declares no log
// directive of its own and so inherits them.
func GetDefaultLogDir(c *gin.Context) {
	dir := ""
	accessLogPath := nginx.GetAccessLogPath()
	if accessLogPath != "" {
		dir = filepath.Dir(accessLogPath)
	} else if prefix := nginx.GetPrefix(); prefix != "" {
		// nginx may not be running or may declare no access_log at all; the
		// logs directory under the nginx prefix is whitelisted too.
		dir = filepath.Join(prefix, "logs")
	}

	c.JSON(http.StatusOK, gin.H{
		"access_log_dir":  dir,
		"access_log_path": usableLogPath(accessLogPath),
		"error_log_path":  usableLogPath(nginx.GetErrorLogPath()),
	})
}

// usableLogPath returns the path when it passes the log whitelist, else "".
func usableLogPath(path string) string {
	if path == "" || !utils.IsValidLogPath(path) {
		return ""
	}
	return path
}
