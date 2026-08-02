package system

import (
	"github.com/gin-gonic/gin"

	"github.com/0xJacky/Nginx-UI/internal/middleware"
)

func InitPublicRouter(r *gin.RouterGroup) {
	r.GET("install", InstallLockCheck)
	r.GET("translation/:code", GetTranslation)
}

func InitSetupRouter(r *gin.RouterGroup) {
	r.POST("install", middleware.EncryptedParams(), InstallNginxUI)

	g := r.Group("self_check", middleware.Proxy())
	g.GET("", SelfCheck)
	g.POST("/:name/fix", SelfCheckFix)
	g.GET("timeout", TimeoutCheck)

	r.GET("self_check/websocket", middleware.ProxyWs(), CheckWebSocket)
}

func InitPrivateRouter(r *gin.RouterGroup) {
	r.GET("upgrade/release", GetRelease)
	r.GET("upgrade/current", GetCurrentVersion)

	r.POST("system/port_scan", PortScan)

	r.Any("system/stats", GetProcessStats)
	// Restarting takes the whole demo container down with it.
	r.Any("system/restart", middleware.RejectInDemo(), Restart)

	g := r.Group("self_check")
	g.GET("", SelfCheck)
	g.POST("/:name/fix", SelfCheckFix)
	g.GET("timeout", TimeoutCheck)
}

func InitWebSocketRouter(r *gin.RouterGroup) {
	// internal/upgrader/binary.go already turns the install step into a no-op in
	// demo mode, but the archive is still downloaded and verified first. Refuse
	// before any of that runs.
	r.GET("upgrade/perform", middleware.RejectInDemo(), PerformCoreUpgrade)
	r.GET("self_check/websocket", CheckWebSocket)
}
