package system

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitPublicRouter(r *gin.RouterGroup) {
	r.GET("install", InstallLockCheck)
	r.POST("install", middleware.EncryptedParams(), InstallNginxUI)
	r.GET("translation/:code", GetTranslation)
}

func InitPrivateRouter(r *gin.RouterGroup) {
	r.GET("upgrade/release", GetRelease)
	r.GET("upgrade/current", GetCurrentVersion)
	r.GET("self_check", SelfCheck)
	r.POST("self_check/:name/fix", SelfCheckFix)

	// Backup endpoint only
	r.GET("system/backup", CreateBackup)
}

func InitBackupRestoreRouter(r *gin.RouterGroup) {
	r.POST("system/backup/restore",
		func(ctx *gin.Context) {
			// If system is installed, verify user authentication
			if installLockStatus() {
				middleware.AuthRequired()(ctx)
			} else {
				ctx.Next()
			}
		},
		middleware.EncryptedForm(),
		RestoreBackup)
}

func InitWebSocketRouter(r *gin.RouterGroup) {
	r.GET("upgrade/perform", PerformCoreUpgrade)
	r.GET("self_check/websocket", CheckWebSocket)
}
