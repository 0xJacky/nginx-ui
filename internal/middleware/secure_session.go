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
	"github.com/uozi-tech/cosy"
	cSettings "github.com/uozi-tech/cosy/settings"
)

const SecureSessionCookieName = "_nginx_ui_secure_session"
const insecureSecureSessionCookieName = SecureSessionCookieName + "_http"

// secureSessionCookieName keeps the HTTPS and HTTP session-binding
// cookies separate. Browsers do not reliably allow an HTTP response to
// replace a Secure cookie left by a previous HTTPS session.
func secureSessionCookieName(https bool) string {
	if https {
		return SecureSessionCookieName
	}
	return insecureSecureSessionCookieName
}

func SecureSessionCookieNameForRequest(c *gin.Context) string {
	return secureSessionCookieName(cSettings.ServerSettings.EnableHTTPS)
}

func ensureSecureSessionCookie(c *gin.Context) {
	cookieName := SecureSessionCookieNameForRequest(c)
	if _, err := c.Cookie(cookieName); err != http.ErrNoCookie {
		return
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		cookieName,
		hex.EncodeToString(b),
		0,
		"/",
		"",
		cSettings.ServerSettings.EnableHTTPS,
		true,
	)
	c.Request.AddCookie(&http.Cookie{Name: cookieName, Value: hex.EncodeToString(b)})
}

// EnsureSecureSessionCookie makes sure the session-binding cookie exists.
func EnsureSecureSessionCookie(c *gin.Context) {
	ensureSecureSessionCookie(c)
}

// SecureSessionCookie sets an HttpOnly SameSite=Lax cookie when serving the SPA.
// The short-token endpoint also requires an authenticated JWT in its header.
func SecureSessionCookie() gin.HandlerFunc {
	return func(c *gin.Context) {
		ensureSecureSessionCookie(c)
		c.Next()
	}
}

const SecureSessionVerifiedKey = "SecureSessionVerified"

func RequireSecureSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if verifySecureSession(c) {
			c.Next()
		}
	}
}

// verifySecureSession checks the caller without advancing Gin's handler chain.
// Proxy uses the same check before replacing the caller with a node principal.
func verifySecureSession(c *gin.Context) bool {
	if _, ok := c.Get(nodeauth.GinPrincipalKey); ok {
		return true
	}
	if token, ok := c.Get(internalmcp.ServiceTokenPrincipalKey); ok {
		principal, valid := token.(*internalmcp.ServiceTokenPrincipal)
		if !valid || !principal.HasScope(model.APITokenScopeWrite) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "Service token write scope is required",
			})
			return false
		}
		return true
	}

	u, ok := c.Get("user")
	if !ok {
		return true
	}
	cUser := u.(*model.User)
	needsSecureSession, err := userNeedsSecureSession(cUser)
	if err != nil {
		cosy.ErrHandler(c, err)
		c.Abort()
		return false
	}
	if !needsSecureSession {
		return true
	}
	ssid := c.GetHeader("X-Secure-Session-ID")
	if ssid == "" {
		ssid = c.Query("X-Secure-Session-ID")
	}
	if ssid == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"message": "Secure Session ID is empty",
		})
		return false
	}
	if user.VerifySecureSessionID(ssid, cUser.ID) {
		c.Set(SecureSessionVerifiedKey, true)
		return true
	}
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"message": "Secure Session ID is invalid",
	})
	return false
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
		if nodeProxyVerifiedTwoFactor(c) {
			return true
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": message})
		return false
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

// RestrictedDirectiveSaveAllowed reports whether this request may save
// configuration holding restricted nginx directives, which load code into
// the Nginx worker. It has to come from an interactive administrator whose
// two-factor session was verified for this very request: the denylist exists
// to stop a stolen JWT, a write-scoped service token, an MCP tool, the AI
// assistant or a cluster node from turning a configuration write into code
// execution, and none of those can produce that verification.
//
// A node principal is refused on purpose, so a configuration with restricted
// directives does not replicate across the cluster. Propagating the
// verification the way nodeProxyVerifiedTwoFactor does would extend
// node-to-node trust to code execution.
//
// The public demo is refused as well. Its local configuration CRUD is open to
// visitors because a configuration write cannot execute code there, which is
// exactly what this hatch would change: the demo password is published, and
// passkey enrollment is reachable with it, so a visitor could reach a
// verified session.
//
// Place it behind RequireSecureSession, which is what sets the verified
// marker. An account with no second factor enrolled never gets the marker,
// so it is refused here as well. The function never aborts the request; the
// caller decides how to report a refusal.
func RestrictedDirectiveSaveAllowed(c *gin.Context) bool {
	if DemoMode() {
		return false
	}
	if _, isNode := c.Get(nodeauth.GinPrincipalKey); isNode {
		return false
	}
	if _, isServiceToken := c.Get(internalmcp.ServiceTokenPrincipalKey); isServiceToken {
		return false
	}

	verified, _ := c.Get(SecureSessionVerifiedKey)

	return verified == true
}

// RestrictedDirectiveStepUp reports what a caller would have to do before it
// may save configuration holding restricted nginx directives.
type RestrictedDirectiveStepUp int

const (
	// RestrictedDirectiveStepUpImpossible is a caller that is never offered
	// the step-up: a cluster node or a service token. It keeps the plain
	// rejection, whatever its account has enrolled.
	RestrictedDirectiveStepUpImpossible RestrictedDirectiveStepUp = iota
	// RestrictedDirectiveStepUpUnavailable is an interactive administrator
	// with no second factor, who has to enrol one before confirming.
	RestrictedDirectiveStepUpUnavailable
	// RestrictedDirectiveStepUpAvailable is an interactive administrator who
	// can confirm the save with a verified two-factor session.
	RestrictedDirectiveStepUpAvailable
)

// RestrictedDirectiveStepUpFor reports which of those three the caller is, so
// a handler can tell an administrator to confirm, tell them to enrol a second
// factor, or keep the plain rejection for a non-interactive caller.
func RestrictedDirectiveStepUpFor(c *gin.Context) (RestrictedDirectiveStepUp, error) {
	// On the demo there is nothing to offer: the hatch stays shut however the
	// visitor authenticates, so they get the plain rejection rather than a
	// confirmation they could never satisfy.
	if DemoMode() {
		return RestrictedDirectiveStepUpImpossible, nil
	}
	if _, isNode := c.Get(nodeauth.GinPrincipalKey); isNode {
		return RestrictedDirectiveStepUpImpossible, nil
	}
	if _, isServiceToken := c.Get(internalmcp.ServiceTokenPrincipalKey); isServiceToken {
		return RestrictedDirectiveStepUpImpossible, nil
	}

	u, ok := c.Get("user")
	if !ok {
		return RestrictedDirectiveStepUpImpossible, nil
	}
	cUser, ok := u.(*model.User)
	if !ok {
		return RestrictedDirectiveStepUpImpossible, nil
	}

	hasSecondFactor, err := userNeedsSecureSession(cUser)
	if err != nil {
		return RestrictedDirectiveStepUpImpossible, err
	}
	if !hasSecondFactor {
		return RestrictedDirectiveStepUpUnavailable, nil
	}

	return RestrictedDirectiveStepUpAvailable, nil
}

// RequireInteractiveUser prevents a verified node principal from invoking
// controller-administration APIs intended only for a logged-in administrator.
func RequireInteractiveUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if verifyInteractiveUser(c) {
			c.Next()
		}
	}
}

func verifyInteractiveUser(c *gin.Context) bool {
	_, isNode := c.Get(nodeauth.GinPrincipalKey)
	_, isServiceToken := c.Get(internalmcp.ServiceTokenPrincipalKey)
	if isNode || isServiceToken {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"message": "This operation requires an interactive administrator",
		})
		return false
	}
	return true
}

// RequireInteractiveUserOrProxy keeps service tokens away from endpoints that
// hand out an interactive session, such as the terminal, while still letting a
// controller reach them on a child node. The controller's proxy signs the
// forwarded request as that node, so the node principal is accepted here; the
// controller has already applied these gates to its own user before proxying.
func RequireInteractiveUserOrProxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := c.Get(nodeauth.GinPrincipalKey); ok {
			c.Next()
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

// RequireNodePrincipal keeps a route to the controller that signed the
// request itself, such as the endpoints that change the credential the
// controller authenticates with. Mount such routes outside Proxy: the node
// proxy signs a forwarded request as the controller, so a forwarded call would
// act on that relationship for whoever selected the node. The guard is
// deliberately absent from proxyGuardAuthorizations, so Proxy refuses to
// forward a route that carries it.
func RequireNodePrincipal() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := c.Get(nodeauth.GinPrincipalKey); !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "This operation is only available to a node controller",
			})
			return
		}
		c.Next()
	}
}

func userNeedsSecureSession(cUser *model.User) (bool, error) {
	if cUser.EnabledOTP() || cUser.EnabledTwoFA {
		return true, nil
	}

	if cUser.ID == 0 {
		return false, nil
	}
	if model.UseDB() == nil {
		return true, nil
	}

	return cUser.EnabledPasskey()
}
