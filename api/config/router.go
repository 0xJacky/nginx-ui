package config

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.RouterGroup) {
	r.GET("config_base_path", GetBasePath)

	r.GET("configs", GetConfigs)
	r.GET("configs/*path", GetConfig)
	r.POST("configs", AddConfig)
	r.POST("configs/*path", EditConfig)

	o := r.Group("", middleware.RequireSecureSession())
	{
		o.POST("config_mkdir", Mkdir)
		o.POST("config_rename", Rename)
		o.POST("config_delete", DeleteConfig)
	}

	r.GET("config_histories", GetConfigHistory)
}
