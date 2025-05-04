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

	r.GET("system/backup", CreateBackup)
	r.GET("system/processing", GetProcessingStatus)
}

func InitSelfCheckRouter(r *gin.RouterGroup) {
	g := r.Group("self_check")
	g.GET("", authIfInstalled, SelfCheck)
	g.POST("/:name/fix", authIfInstalled, SelfCheckFix)
	g.GET("websocket", authIfInstalled, CheckWebSocket)
}

func InitBackupRestoreRouter(r *gin.RouterGroup) {
	r.POST("system/backup/restore",
		authIfInstalled,
		middleware.EncryptedForm(),
		RestoreBackup)
}

func InitWebSocketRouter(r *gin.RouterGroup) {
	r.GET("upgrade/perform", PerformCoreUpgrade)
}
