package terminal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newTerminalRouter(authenticate gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	InitRouter(router.Group("/", authenticate))
	return router
}

func servePty(router http.Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/pty", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func otpUser() *model.User {
	return &model.User{
		Model:     model.Model{ID: 1},
		Name:      "otp",
		Status:    true,
		OTPSecret: []byte("otp-enabled"),
	}
}

func TestTerminalRequiresSecureSessionForOTPUser(t *testing.T) {
	router := newTerminalRouter(func(c *gin.Context) {
		c.Set("user", otpUser())
		c.Next()
	})

	require.Equal(t, http.StatusUnauthorized, servePty(router).Code)
}

// A controller must verify its own user's secure session before it proxies the
// terminal to a child node. ProxyWs would reach for the database here, so an
// unverified request has to be stopped before it.
func TestTerminalChecksSecureSessionBeforeProxying(t *testing.T) {
	router := newTerminalRouter(func(c *gin.Context) {
		c.Set("user", otpUser())
		c.Set("ProxyNodeID", "1")
		c.Next()
	})

	require.Equal(t, http.StatusUnauthorized, servePty(router).Code)
}

// On the child node the proxied request is signed as that node. It must reach
// the handler, which rejects this plain GET as a failed WebSocket upgrade.
func TestTerminalAcceptsProxiedNodeRequest(t *testing.T) {
	router := newTerminalRouter(func(c *gin.Context) {
		c.Set(nodeauth.GinPrincipalKey, &nodeauth.Principal{
			CredentialID: "credential",
			AuthMethod:   model.NodeAuthMethodPaired,
		})
		c.Next()
	})

	require.Equal(t, http.StatusBadRequest, servePty(router).Code)
}

func TestTerminalRejectsServiceToken(t *testing.T) {
	router := newTerminalRouter(func(c *gin.Context) {
		c.Set(internalmcp.ServiceTokenPrincipalKey, &internalmcp.ServiceTokenPrincipal{
			Name:   "writer",
			Scopes: []string{model.APITokenScopeWrite},
		})
		c.Next()
	})

	require.Equal(t, http.StatusForbidden, servePty(router).Code)
}
