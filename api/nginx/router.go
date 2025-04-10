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
	r.POST("nginx/test", Test)
	r.GET("nginx/status", Status)
	// 获取 Nginx 详细状态信息，包括连接数、进程信息等（Issue #850）
	r.GET("nginx/detail_status", GetDetailStatus)
	// 使用SSE推送Nginx详细状态信息
	r.GET("nginx/detail_status/stream", StreamDetailStatus)
	// 获取 stub_status 模块状态
	r.GET("nginx/stub_status", CheckStubStatus)
	// 启用或禁用 stub_status 模块
	r.POST("nginx/stub_status", ToggleStubStatus)
	r.POST("nginx_log", nginx_log.GetNginxLogPage)
	r.GET("nginx/directives", GetDirectives)
}
