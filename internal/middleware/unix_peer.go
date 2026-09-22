package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// unixPeerAddr is the address reported for a client that connected directly
// to the Unix socket. Such a client is a local process, exactly like a TCP
// client on the loopback interface, so it is treated the same way by the
// IP allowlist, login attempt tracking and audit records.
const unixPeerAddr = "127.0.0.1"

// UnixPeerAddr normalizes requests that arrived over a Unix socket without a
// forwarded client address. Go reports such peers with an empty or "@"
// RemoteAddr and gin's ClientIP then yields "<nil>", which fails the IP
// allowlist, collapses every such client into one login ban bucket and
// produces unusable audit records. Requests that carry X-Forwarded-For or
// X-Real-IP from the reverse proxy are left untouched: gin already trusts a
// Unix socket peer for those headers.
func UnixPeerAddr() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isUnixPeer(c.Request) {
			c.Request.RemoteAddr = net.JoinHostPort(unixPeerAddr, "0")
			if c.Request.Header.Get("X-Forwarded-For") == "" && c.Request.Header.Get("X-Real-IP") == "" {
				c.Request.Header.Set("X-Real-IP", unixPeerAddr)
			}
		}
		c.Next()
	}
}

func isUnixPeer(r *http.Request) bool {
	if _, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil {
		return false
	}
	localAddr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	return ok && strings.HasPrefix(localAddr.Network(), "unix")
}
