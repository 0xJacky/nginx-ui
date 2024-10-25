package cluster

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	// Environment
	r.GET("environments", GetEnvironmentList)
	r.POST("environments/load_from_settings", LoadEnvironmentFromSettings)
	envGroup := r.Group("environments")
	{
		envGroup.GET("/:id", GetEnvironment)
		envGroup.POST("", AddEnvironment)
		envGroup.POST("/:id", EditEnvironment)
		envGroup.DELETE("/:id", DeleteEnvironment)
	}
	// Node
	r.GET("node", GetCurrentNode)
}
