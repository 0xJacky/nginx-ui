package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	cSettings "github.com/uozi-tech/cosy/settings"
)

const SecureSessionCookieName = "_nginx_ui_secure_session"

func ensureSecureSessionCookie(c *gin.Context) {
	if _, err := c.Cookie(SecureSessionCookieName); err != http.ErrNoCookie {
		return
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		SecureSessionCookieName,
		hex.EncodeToString(b),
		0,
		"/",
		"",
		cSettings.ServerSettings.EnableHTTPS,
		true,
	)
}

// EnsureSecureSessionCookie makes sure the session-binding cookie exists.
func EnsureSecureSessionCookie(c *gin.Context) {
	ensureSecureSessionCookie(c)
}

// SecureSessionCookie sets an HttpOnly SameSite=Lax cookie when serving the SPA.
// This cookie acts as a CSRF-proof session binding for the short token endpoint.
func SecureSessionCookie() gin.HandlerFunc {
	return func(c *gin.Context) {
		ensureSecureSessionCookie(c)
		c.Next()
	}
}

const SecureSessionVerifiedKey = "SecureSessionVerified"

func RequireSecureSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := c.Get(nodeauth.GinPrincipalKey); ok {
			c.Next()
			return
		}
		if token, ok := c.Get(internalmcp.ServiceTokenPrincipalKey); ok {
			principal, valid := token.(*internalmcp.ServiceTokenPrincipal)
			if !valid || !principal.HasScope(model.APITokenScopeWrite) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"message": "Service token write scope is required",
				})
				return
			}
			c.Next()
			return
		}

		u, ok := c.Get("user")
		if !ok {
			c.Next()
			return
		}
		cUser := u.(*model.User)
		if !userNeedsSecureSession(cUser) {
			c.Next()
			return
		}
		ssid := c.GetHeader("X-Secure-Session-ID")
		if ssid == "" {
			ssid = c.Query("X-Secure-Session-ID")
		}
		if ssid == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Secure Session ID is empty",
			})
			return
		}

		if user.VerifySecureSessionID(ssid, cUser.ID) {
			c.Set(SecureSessionVerifiedKey, true)
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"message": "Secure Session ID is invalid",
		})
	}
}

// VerifiedSecureSession reports whether RequireSecureSession marked the
// request as carrying a verified two-factor session. When it did not, the
// request is aborted with 401 and the given message, so the caller only has
// to return.
func VerifiedSecureSession(c *gin.Context, message string) bool {
	if verified, _ := c.Get(SecureSessionVerifiedKey); verified != true {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"message": message,
		})
		return false
	}

	return true
}

// VerifiedTwoFactorOrProxy gates handlers that change the nginx control
// target or the SSH material behind it. A controller configuring a child node
// reaches them through Proxy(), which signs the forwarded request as that
// node, so a node principal is accepted the same way RequireSecureSession
// does. Rejecting it would surface as an opaque 503, because the proxy
// rewrites 403 responses. Everyone else needs a verified two-factor session.
func VerifiedTwoFactorOrProxy(c *gin.Context, message string) bool {
	if _, ok := c.Get(nodeauth.GinPrincipalKey); ok {
		return true
	}

	return VerifiedSecureSession(c, message)
}

// RequireVerifiedTwoFactorOrProxy is the middleware form of
// VerifiedTwoFactorOrProxy. Place it after RequireSecureSession, which is what
// sets the verified marker.
func RequireVerifiedTwoFactorOrProxy(message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !VerifiedTwoFactorOrProxy(c, message) {
			return
		}
		c.Next()
	}
}

// RequireInteractiveUser prevents a verified node principal from invoking
// controller-administration APIs intended only for a logged-in administrator.
func RequireInteractiveUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := c.Get(nodeauth.GinPrincipalKey); ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "This operation requires an interactive administrator",
			})
			return
		}
		if _, ok := c.Get(internalmcp.ServiceTokenPrincipalKey); ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "This operation requires an interactive administrator",
			})
			return
		}
		c.Next()
	}
}

func userNeedsSecureSession(cUser *model.User) bool {
	if cUser.EnabledOTP() || cUser.EnabledTwoFA {
		return true
	}

	if cUser.ID == 0 || model.UseDB() == nil {
		return false
	}

	return cUser.EnabledPasskey()
}
