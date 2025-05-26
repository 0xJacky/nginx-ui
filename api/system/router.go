package system

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func authIfInstalled(ctx *gin.Context) {
	if installLockStatus() || isInstallTimeoutExceeded() {
		middleware.AuthRequired()(ctx)
	} else {
		ctx.Next()
	}
}

func InitPublicRouter(r *gin.RouterGroup) {
	r.GET("install", InstallLockCheck)
	r.POST("install", middleware.EncryptedParams(), InstallNginxUI)
	r.GET("translation/:code", GetTranslation)
}

func InitPrivateRouter(r *gin.RouterGroup) {
	r.GET("upgrade/release", GetRelease)
	r.GET("upgrade/current", GetCurrentVersion)

	r.GET("system/processing", GetProcessingStatus)
	r.POST("system/port_scan", PortScan)
}

func InitSelfCheckRouter(r *gin.RouterGroup) {
	g := r.Group("self_check", authIfInstalled)
	g.GET("", middleware.Proxy(), SelfCheck)
	g.POST("/:name/fix", middleware.Proxy(), SelfCheckFix)
	g.GET("sse", middleware.Proxy(), CheckSSE)
	g.GET("websocket", middleware.ProxyWs(), CheckWebSocket)
}

func InitBackupRestoreRouter(r *gin.RouterGroup) {
	// Backup and restore routes moved to api/backup package
}

func InitWebSocketRouter(r *gin.RouterGroup) {
	r.GET("upgrade/perform", PerformCoreUpgrade)
}
