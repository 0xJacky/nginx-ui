package config

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("config_base_path", GetBasePath)

	r.GET("configs", GetConfigs)
	r.GET("config/*name", GetConfig)
	r.POST("config", AddConfig)
	r.POST("config/*name", EditConfig)
	r.POST("config_mkdir", Mkdir)
	r.POST("config_rename", Rename)
}
