package settings

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.RouterGroup) {
	r.GET("settings/server/name", GetServerName)
	r.GET("settings", GetSettings)
	r.POST("settings", middleware.RequireSecureSession(), SaveSettings)

	r.GET("settings/auth/banned_ips", GetBanLoginIP)
	r.DELETE("settings/auth/banned_ip", RemoveBannedIP)
}
