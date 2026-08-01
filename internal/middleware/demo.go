package middleware

import (
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// RejectInDemo blocks a route outright when the node runs in demo mode.
//
// It is meant for routes whose side effects escape the container: ACME
// issuance against a real CA, backup upload to object storage, outbound
// webhooks, process restart, and settings writes that could redirect any of
// the above. Local CRUD (sites, streams, configs) is deliberately left open,
// because the demo container runs on ephemeral disk and restores itself.
//
// Note this is not redundant with RequireSecureSession: that middleware is a
// pass-through for users with no OTP, 2FA or passkey, and demo mode refuses
// TOTP enrollment, so the demo user can never trigger it.
func RejectInDemo() gin.HandlerFunc {
	return func(c *gin.Context) {
		if settings.NodeSettings.Demo {
			cosy.ErrHandler(c, ErrDisabledInDemo)
			c.Abort()
			return
		}

		c.Next()
	}
}

// DemoReadOnly blocks state-changing methods in demo mode while leaving reads
// intact. Use it on cosy CRUD groups where the listing is worth showing but
// the writes are not worth accepting.
func DemoReadOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if settings.NodeSettings.Demo && !isDemoSafeMethod(c.Request.Method) {
			cosy.ErrHandler(c, ErrDisabledInDemo)
			c.Abort()
			return
		}

		c.Next()
	}
}

func isDemoSafeMethod(method string) bool {
	switch method {
	case "GET", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}
