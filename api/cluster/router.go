package cluster

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.RouterGroup) {
	// The node secret is the only thing a relationship starts from, so an
	// upgrade arrives already authenticated by the middleware like any other
	// node request.
	r.POST("node/pair/upgrade", UpgradeLegacyPairing)

	// Node
	r.GET("nodes", GetNodeList)
	nodeGroup := r.Group("nodes")
	{
		nodeGroup.GET("/:id", GetNode)
	}

	admin := r.Group("", middleware.RequireInteractiveUser(), middleware.RequireSecureSession())
	{
		admin.POST("nodes", AddNode)
		admin.POST("nodes/:id", EditNode)
		admin.PATCH("nodes/:id", RecoverNode)
		admin.DELETE("nodes/:id", DeleteNode)
		admin.POST("nodes/load_from_settings", LoadNodeFromSettings)
		admin.GET("nodes/:id/secret", GetNodeSecret)
		admin.GET("nodes/:id/credentials", GetNodeCredentials)
		admin.POST("nodes/:id/credentials/rotate", RotateNodeCredential)
		admin.POST("nodes/:id/auth-upgrade/retry", RetryNodeAuthUpgrade)
		admin.GET("node/credentials", ListControllerCredentials)
		admin.DELETE("node/credentials/:credential_id", RevokeControllerCredential)
	}

	r.POST("node/credentials/:credential_id/rotation", BeginControllerCredentialRotation)
	r.POST("node/credentials/:credential_id/rotation/confirm", ConfirmControllerCredentialRotation)

	mutations := r.Group("", middleware.RequireSecureSession())
	{
		mutations.POST("nodes/reload_nginx", ReloadNginx)
		mutations.POST("nodes/restart_nginx", RestartNginx)
		mutations.POST("nodes/sync", SyncNodes)
		mutations.POST("namespace/sync", UpsertNamespace)
		mutations.POST("namespaces/:id/sync", SyncNamespace)
		mutations.POST("namespaces", AddNamespace)
		mutations.POST("namespaces/:id", ModifyNamespace)
		mutations.DELETE("namespaces/:id", DeleteNamespace)
		mutations.POST("namespaces/:id/recover", RecoverNamespace)
		mutations.POST("namespaces/order", UpdateNamespacesOrder)
	}

	r.GET("namespaces", GetNamespaceList)
	r.GET("namespaces/:id", GetNamespace)
}

func InitWebSocketRouter(r *gin.RouterGroup) {
	r.GET("nodes/enabled", GetAllEnabledNodeWS)
}
