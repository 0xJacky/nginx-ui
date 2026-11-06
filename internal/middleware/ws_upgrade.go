package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// IsWebSocketUpgrade reports whether the request asks to switch to the
// WebSocket protocol.
func IsWebSocketUpgrade(r *http.Request) bool {
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return false
	}
	for _, part := range strings.Split(r.Header.Get("Connection"), ",") {
		if strings.EqualFold(strings.TrimSpace(part), "upgrade") {
			return true
		}
	}
	return false
}

// ForWebSocketUpgrade picks a middleware per request: upgrade requests run
// upgrade, every other request runs regular. It lets one route serve both
// header authenticated calls and browser WebSockets, which can only carry
// their credentials in the query string.
func ForWebSocketUpgrade(upgrade, regular gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsWebSocketUpgrade(c.Request) {
			upgrade(c)
			return
		}
		regular(c)
	}
}

// RequireWebSocketOrigin rejects an upgrade from an origin the host does not
// trust, the check the host's own WebSocket handlers run when they upgrade.
// Routes that proxy the upgrade elsewhere never upgrade themselves, so they
// need it as a middleware.
func RequireWebSocketOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !CheckWebSocketOrigin(c.Request) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}
