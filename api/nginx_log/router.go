package nginx_log

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

// InitRouter registers all the nginx log related routes
func InitRouter(r *gin.RouterGroup) {

	r.GET("nginx_logs", GetLogList)
	r.POST("nginx_log/page", GetNginxLogPage)
	r.POST("nginx_log/analytics", GetLogAnalytics)
	r.GET("nginx_log/entries", GetLogEntries)
	r.POST("nginx_log/search", AdvancedSearchLogs)
	r.GET("nginx_log/preflight", GetLogPreflight)
	r.POST("nginx_log/dashboard", GetDashboardAnalytics)
	r.POST("nginx_log/geo/world", GetWorldMapData)
	r.POST("nginx_log/geo/china", GetChinaMapData)
	r.POST("nginx_log/geo/china/city", GetChinaCityMapData)
	r.POST("nginx_log/geo/stats", GetGeoStats)
	r.POST("nginx_log/index/rebuild", middleware.RequireSecureSession(), RebuildIndex)
	r.POST("nginx_log/settings/advanced_indexing/enable", middleware.RequireSecureSession(), EnableAdvancedIndexing)
	r.POST("nginx_log/settings/advanced_indexing/disable", middleware.RequireSecureSession(), DisableAdvancedIndexing)
	r.GET("nginx_log/settings/advanced_indexing/status", GetAdvancedIndexingStatus)
	r.GET("nginx_log/default_log_dir", GetDefaultLogDir)
}

func InitPublicRouter(r *gin.RouterGroup) {
	r.GET("nginx_log/geo/boundary/:filename", GetGeoBoundaryFile)
}

func InitWebSocketRouter(r *gin.RouterGroup) {
	r.GET("nginx_log", Log)
}
