package cluster

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	// Environment
	r.GET("environments", GetEnvironmentList)
	r.GET("environments/enabled", GetAllEnabledEnvironmentWS)
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

	r.POST("environments/reload_nginx", ReloadNginx)
	r.POST("environments/restart_nginx", RestartNginx)

	r.GET("env_groups", GetGroupList)
	r.GET("env_groups/:id", GetGroup)
	r.POST("env_groups", AddGroup)
	r.POST("env_groups/:id", ModifyGroup)
	r.DELETE("env_groups/:id", DeleteGroup)
	r.POST("env_groups/:id/recover", RecoverGroup)
	r.POST("env_groups/order", UpdateGroupsOrder)
}
