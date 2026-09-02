package settings

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.RouterGroup) {
	r.GET("settings/server/name", GetServerName)
	r.GET("settings", GetSettings)
	r.GET("settings/protected", middleware.RequireInteractiveUser(), middleware.RequireSecureSession(), GetProtectedSetting)
	// Demo mode itself is protected:"true", but everything around it is not:
	// a settings write can repoint the ACME CA away from staging, swap the
	// OpenAI base URL, or change the log directory whitelist.
	r.POST("settings", middleware.RequireSecureSession(), middleware.RejectInDemo(), SaveSettings)
	// The nginx control target and its private key decide which machine the
	// nginx commands run on, so they get the same guards as the settings write.
	r.POST("settings/nginx/control", middleware.RequireSecureSession(), middleware.RejectInDemo(), SaveNginxControlSettings)
	r.POST("settings/nginx/private-key", middleware.RequireSecureSession(), middleware.RejectInDemo(), SaveNginxPrivateKey)

	r.GET("settings/auth/banned_ips", GetBanLoginIP)
	r.DELETE("settings/auth/banned_ip", middleware.RequireSecureSession(), RemoveBannedIP)
}
