package ingress

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

// InitRouter registers the ingress API. See docs/guide/ingress.md.
func InitRouter(r *gin.RouterGroup) {
	r.GET("/ingresses", ListIngresses)
	r.GET("/ingresses/:name", GetIngress)

	o := r.Group("", middleware.RequireSecureSession())
	{
		o.PUT("/ingresses", SyncIngresses)
		o.PUT("/ingresses/:name", PutIngress)
		o.DELETE("/ingresses/:name", DeleteIngress)
		o.PATCH("/ingresses/:name/backends/:backend/servers", PatchIngressServer)
		o.POST("/ingresses/:name/reconcile", ReconcileIngress)
	}
}
