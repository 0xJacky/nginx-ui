package nginx_log

import "github.com/gin-gonic/gin"

// InitRouter registers all the nginx log related routes
func InitRouter(r *gin.RouterGroup) {

	r.GET("nginx_logs", GetLogList)
	r.POST("nginx_log/page", GetNginxLogPage)
	r.GET("nginx_log/default_log_dir", GetDefaultLogDir)
	r.GET("nginx_log/legacy_indexing", GetLegacyIndexingStatus)
}

func InitWebSocketRouter(r *gin.RouterGroup) {
	r.GET("nginx_log", Log)
}
