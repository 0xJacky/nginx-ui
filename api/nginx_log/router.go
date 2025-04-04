package nginx_log

import "github.com/gin-gonic/gin"

// InitRouter registers all the nginx log related routes
func InitRouter(r *gin.RouterGroup) {
	r.GET("nginx_log", Log)
	r.GET("nginx_logs", GetLogList)
}
