package nginx_log

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("nginx_log", Log)
	r.GET("nginx_logs", GetLogList)
	r.GET("nginx_logs/index_status", GetNginxLogsLive)
}
