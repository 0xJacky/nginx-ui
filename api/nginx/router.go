package nginx

import (
	"github.com/0xJacky/Nginx-UI/api/nginx_log"
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.RouterGroup) {
	r.POST("ngx/build_config", BuildNginxConfig)
	r.POST("ngx/tokenize_config", TokenizeNginxConfig)
	r.POST("ngx/format_code", FormatNginxConfig)
	r.POST("nginx/reload", Reload)
	r.POST("nginx/restart", Restart)
	r.POST("nginx/test", TestConfig)
	r.GET("nginx/status", Status)
	// Get detailed Nginx status information, including connection count, process information, etc. (Issue #850)
	r.GET("nginx/detail_status", GetDetailStatus)
	// Use SSE to push detailed Nginx status information
	r.GET("nginx/detail_status/stream", StreamDetailStatus)
	// Get stub_status module status
	r.GET("nginx/stub_status", CheckStubStatus)
	// Enable or disable stub_status module
	r.POST("nginx/stub_status", ToggleStubStatus)
	r.POST("nginx_log", nginx_log.GetNginxLogPage)
	r.GET("nginx/directives", GetDirectives)

	// Performance optimization endpoints
	r.GET("nginx/performance", GetPerformanceSettings)
	r.POST("nginx/performance", UpdatePerformanceSettings)

	r.GET("nginx/modules", GetModules)
}
