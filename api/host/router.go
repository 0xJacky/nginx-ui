package host

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

// InitRouter registers the host setup wizard endpoints. They read operator
// supplied filesystem paths and open SSH sessions, so they require the same
// secure session as the protected nginx control settings.
//
// The group stays reachable by a node principal on purpose: it sits under
// middleware.Proxy(), so a controller configuring a child node arrives here
// signed as that node. RequireInteractiveUser() would turn that flow into an
// opaque 503, because the proxy rewrites 403 responses.
func InitRouter(r *gin.RouterGroup) {
	g := r.Group("host/setup", middleware.RequireSecureSession())
	{
		g.GET("preview", Preview)
		g.POST("preview", Preview)
		g.POST("keypair", GenerateKeypair)
		g.GET("publickey", GetPublicKey)
		g.DELETE("keypair", DeleteKeypair)
		g.GET("ssh-targets", SSHTargets)
		g.POST("connection", TestConnection)
		g.POST("discover", Discover)
		g.POST("diagnose", Diagnose)
		g.POST("verify", Verify)
		g.POST("known-host", TrustHostKey)
		g.POST("host-key/scan", ScanHostKey)
		g.POST("host-key/trust", TrustScannedHostKey)
		g.POST("host-key/replace", ReplaceHostKey)
		g.DELETE("host-key", DeleteHostKey)
	}
}
