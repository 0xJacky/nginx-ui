package cluster

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	// Node
	r.GET("nodes", GetNodeList)
	r.POST("nodes/load_from_settings", LoadNodeFromSettings)
	nodeGroup := r.Group("nodes")
	{
		nodeGroup.GET("/:id", GetNode)
		nodeGroup.POST("", AddNode)
		nodeGroup.POST("/:id", EditNode)
		nodeGroup.DELETE("/:id", DeleteNode)
	}

	r.POST("nodes/reload_nginx", ReloadNginx)
	r.POST("nodes/restart_nginx", RestartNginx)

	r.GET("namespaces", GetNamespaceList)
	r.GET("namespaces/:id", GetNamespace)
	r.POST("namespaces", AddNamespace)
	r.POST("namespaces/:id", ModifyNamespace)
	r.DELETE("namespaces/:id", DeleteNamespace)
	r.POST("namespaces/:id/recover", RecoverNamespace)
	r.POST("namespaces/order", UpdateNamespacesOrder)
}

func InitWebSocketRouter(r *gin.RouterGroup) {
	r.GET("nodes/enabled", GetAllEnabledNodeWS)
}
