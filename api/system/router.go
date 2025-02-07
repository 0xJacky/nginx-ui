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
}

func InitWebSocketRouter(r *gin.RouterGroup) {
	r.GET("upgrade/perform", PerformCoreUpgrade)
	r.GET("self_check/websocket", CheckWebSocket)
}
