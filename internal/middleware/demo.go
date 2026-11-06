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
// TOTP enrollment, so the demo user can never trigger it. Passkey enrollment
// has no such refusal, so anything that a verified session would unlock has
// to check demo mode itself; RestrictedDirectiveSaveAllowed does.
func RejectInDemo() gin.HandlerFunc {
	return func(c *gin.Context) {
		if BlockInDemo(c) {
			return
		}

		c.Next()
	}
}

// DemoMode reports whether this node runs as the public demo, for a caller
// that refuses an action itself instead of aborting the request here. It
// keeps the flag read in this file, which is the one place in the package
// allowed to look at it.
func DemoMode() bool {
	return settings.NodeSettings.Demo
}

// BlockInDemo rejects an action that cannot run on the public demo.
func BlockInDemo(c *gin.Context) bool {
	if !settings.NodeSettings.Demo {
		return false
	}
	cosy.ErrHandler(c, ErrDisabledInDemo)
	c.Abort()
	return true
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
