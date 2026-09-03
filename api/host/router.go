package host

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

const hostSetupTwoFactorMessage = "Two-factor authentication is required to change the host SSH setup"

// InitRouter registers the host setup wizard endpoints. They read operator
// supplied filesystem paths and open SSH sessions, so they require the same
// secure session as the protected nginx control settings, and they are
// unavailable in demo mode like the settings writes they prepare.
//
// The group stays reachable by a node principal on purpose: it sits under
// middleware.Proxy(), so a controller configuring a child node arrives here
// signed as that node. RequireInteractiveUser() would turn that flow into an
// opaque 503, because the proxy rewrites 403 responses.
func InitRouter(r *gin.RouterGroup) {
	g := r.Group("host/setup", middleware.RejectInDemo(), middleware.RequireSecureSession())
	{
		// Read-only rendering of the snippets for the parameters the operator
		// typed; nothing is stored and no connection is opened.
		g.GET("preview", Preview)
		g.POST("preview", Preview)
		// GetPublicKey only reads the default or configured key without a
		// verified session; any other path is gated inside the handler.
		g.GET("publickey", GetPublicKey)
	}

	// Everything that writes SSH material into the container or opens an
	// outbound connection from it needs the same verified two-factor session
	// as POST settings/nginx/control, which is where the result ends up.
	protected := g.Group("", middleware.RequireVerifiedTwoFactorOrProxy(hostSetupTwoFactorMessage))
	{
		protected.POST("keypair", GenerateKeypair)
		protected.DELETE("keypair", DeleteKeypair)
		protected.GET("ssh-targets", SSHTargets)
		protected.POST("connection", TestConnection)
		protected.POST("discover", Discover)
		protected.POST("diagnose", Diagnose)
		protected.POST("verify", Verify)
		protected.POST("host-key/scan", ScanHostKey)
		protected.POST("host-key/trust", TrustScannedHostKey)
		protected.POST("host-key/replace", ReplaceHostKey)
		protected.DELETE("host-key", DeleteHostKey)
	}
}
