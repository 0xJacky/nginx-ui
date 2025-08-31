package nginx_log

import "github.com/gin-gonic/gin"

// InitRouter registers all the nginx log related routes
func InitRouter(r *gin.RouterGroup) {
	r.GET("nginx_log", Log)
	r.GET("nginx_logs", GetLogList)
	r.POST("nginx_log/page", GetNginxLogPage)
	r.POST("nginx_log/analytics", GetLogAnalytics)
	r.GET("nginx_log/entries", GetLogEntries)
	r.POST("nginx_log/search", AdvancedSearchLogs)
	r.GET("nginx_log/preflight", GetLogPreflight)
	r.POST("nginx_log/dashboard", GetDashboardAnalytics)
	r.POST("nginx_log/geo/world", GetWorldMapData)
	r.POST("nginx_log/geo/china", GetChinaMapData)
	r.POST("nginx_log/geo/stats", GetGeoStats)
	r.POST("nginx_log/index/rebuild", RebuildIndex)
	r.POST("nginx_log/settings/advanced_indexing/enable", EnableAdvancedIndexing)
	r.POST("nginx_log/settings/advanced_indexing/disable", DisableAdvancedIndexing)
	r.GET("nginx_log/settings/advanced_indexing/status", GetAdvancedIndexingStatus)
}
