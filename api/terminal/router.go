package terminal

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

// InitRouter registers the PTY endpoint. The group must not carry ProxyWs():
// the gates run first so the controller checks its own user's secure session,
// and only then is a request for a child node proxied. On the child the
// request arrives signed as that node and passes straight to the handler.
func InitRouter(r *gin.RouterGroup) {
	r.GET("pty",
		middleware.RequireInteractiveUserOrProxy(),
		middleware.RequireSecureSession(),
		middleware.ProxyWs(),
		Pty,
	)
}
