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
}

func InitSelfCheckRouter(r *gin.RouterGroup) {
	r.GET("self_check", authIfInstalled, SelfCheck)
	r.POST("self_check/:name/fix", authIfInstalled, SelfCheckFix)
}

func InitBackupRestoreRouter(r *gin.RouterGroup) {
	r.POST("system/backup/restore",
		authIfInstalled,
		middleware.EncryptedForm(),
		RestoreBackup)
}

func InitWebSocketRouter(r *gin.RouterGroup) {
	r.GET("upgrade/perform", PerformCoreUpgrade)
	r.GET("self_check/websocket", CheckWebSocket)
}
