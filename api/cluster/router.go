package cluster

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	// Environment
	r.GET("environments", GetEnvironmentList)
	envGroup := r.Group("environment")
	{
		envGroup.GET("/:id", GetEnvironment)
		envGroup.POST("", AddEnvironment)
		envGroup.POST("/:id", EditEnvironment)
		envGroup.DELETE("/:id", DeleteEnvironment)
	}
	// Node
	r.GET("node", GetCurrentNode)
}
