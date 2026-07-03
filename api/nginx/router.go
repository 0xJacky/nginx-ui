package nginx

import (
	"github.com/0xJacky/Nginx-UI/api/nginx_log"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.RouterGroup) {
	r.POST("ngx/build_config", BuildNginxConfig)
	r.POST("ngx/tokenize_config", TokenizeNginxConfig)
	r.POST("ngx/format_code", FormatNginxConfig)
	r.POST("nginx/test", TestConfig)
	r.POST("nginx/test_namespace", TestConfigWithNamespace)
	r.GET("nginx/status", Status)
	// Get detailed Nginx status information, including connection count, process information, etc. (Issue #850)
	r.GET("nginx/detail_status", GetDetailStatus)
	// Get stub_status module status
	r.GET("nginx/stub_status", CheckStubStatus)
	r.POST("nginx_log", nginx_log.GetNginxLogPage)
	r.GET("nginx/directives", GetDirectives)

	// Performance optimization endpoints
	r.GET("nginx/performance", GetPerformanceSettings)

	r.GET("nginx/modules", GetModules)

	o := r.Group("", middleware.RequireSecureSession())
	{
		o.POST("nginx/reload", Reload)
		o.POST("nginx/restart", Restart)
		// Enable or disable stub_status module
		o.POST("nginx/stub_status", ToggleStubStatus)
		o.POST("nginx/performance", UpdatePerformanceSettings)
		o.POST("nginx/modules/refresh", RefreshModulesCache)
	}
}

func InitWebSocketRouter(r *gin.RouterGroup) {
	r.GET("nginx/detail_status/ws", StreamDetailStatusWS)
}
