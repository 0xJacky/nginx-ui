package settings

import (
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.RouterGroup) {
	r.GET("settings/server/name", GetServerName)
	r.GET("settings", GetSettings)
	r.POST("settings", SaveSettings)

	r.GET("settings/auth/banned_ips", GetBanLoginIP)
	r.DELETE("settings/auth/banned_ip", RemoveBannedIP)
}
