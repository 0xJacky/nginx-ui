package cluster

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitPublicRouter(r *gin.RouterGroup) {
	r.POST("node/pair/complete", CompletePairing)
}

func InitRouter(r *gin.RouterGroup) {
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
		admin.DELETE("nodes/:id", DeleteNode)
		admin.POST("nodes/load_from_settings", LoadNodeFromSettings)
		admin.POST("nodes/:id/pair", PairNode)
		admin.GET("nodes/:id/credentials", GetNodeCredentials)
		admin.POST("nodes/:id/credentials/rotate", RotateNodeCredential)
		admin.POST("node/pairing-codes", CreatePairingCode)
		admin.GET("node/credentials", ListControllerCredentials)
		admin.DELETE("node/credentials/:credential_id", RevokeControllerCredential)
	}

	r.POST("node/credentials/:credential_id/rotation", BeginControllerCredentialRotation)
	r.POST("node/credentials/:credential_id/rotation/confirm", ConfirmControllerCredentialRotation)

	mutations := r.Group("", middleware.RequireSecureSession())
	{
		mutations.POST("nodes/reload_nginx", ReloadNginx)
		mutations.POST("nodes/restart_nginx", RestartNginx)
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
