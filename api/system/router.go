package system

import (
    "github.com/gin-gonic/gin"
)

func InitPublicRouter(r *gin.RouterGroup) {
    r.GET("install", InstallLockCheck)
    r.POST("install", InstallNginxUI)
    r.GET("translation/:code", GetTranslation)
}

func InitPrivateRouter(r *gin.RouterGroup) {
    r.GET("settings", GetSettings)
    r.POST("settings", SaveSettings)

    r.GET("upgrade/release", GetRelease)
    r.GET("upgrade/current", GetCurrentVersion)
    r.GET("upgrade/perform", PerformCoreUpgrade)
}
