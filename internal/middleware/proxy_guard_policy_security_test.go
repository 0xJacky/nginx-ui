package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// RequireSyntheticGuardForProxyTest stands in for a route guard added to this
// package without teaching Proxy how to enforce it.
func RequireSyntheticGuardForProxyTest() gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

func TestProxyGuardPolicyFromHandlerNames(t *testing.T) {
	names := func(handlers ...gin.HandlerFunc) []string {
		router := gin.New()
		var got []string
		router.GET("/probe", append([]gin.HandlerFunc{func(c *gin.Context) { got = c.HandlerNames() }}, handlers...)...)
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/probe", nil))
		return got
	}
	sentinel := func(c *gin.Context) {}
	for _, tc := range []struct {
		name     string
		handlers []gin.HandlerFunc
		want     proxyAuthorization
		known    bool
	}{
		{"unguarded", []gin.HandlerFunc{RejectInDemo(), sentinel}, 0, true},
		{"secure session", []gin.HandlerFunc{RequireSecureSession(), RejectInDemo(), sentinel}, proxySecureSession, true},
		{"interactive", []gin.HandlerFunc{RequireInteractiveUser(), RequireSecureSession(), sentinel}, proxyInteractiveUser | proxySecureSession, true},
		{"verified", []gin.HandlerFunc{RequireSecureSession(), RequireVerifiedTwoFactorOrProxy("verified"), sentinel}, proxySecureSession | proxyVerifiedTwoFactor, true},
		{"interactive or node", []gin.HandlerFunc{RequireInteractiveUserOrProxy(), sentinel}, proxyInteractiveUserOrNode, true},
		{"origin check", []gin.HandlerFunc{RequireWebSocketOrigin(), sentinel}, 0, true},
		{"unknown guard", []gin.HandlerFunc{RequireSecureSession(), RequireSyntheticGuardForProxyTest(), sentinel}, 0, false},
		{"node principal only", []gin.HandlerFunc{RequireNodePrincipal(), sentinel}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy, known := proxyGuardPolicy(names(tc.handlers...))
			require.Equal(t, tc.known, known)
			require.Equal(t, tc.want, policy)
		})
	}
}

// Routes deliberately absent from proxyRouteAuthorizations: their registered
// guards are enforced against the original caller before forwarding, and a
// guard Proxy cannot enforce refuses the remote request outright.
func TestProxyEnforcesRegisteredGuardsWithoutPolicyEntry(t *testing.T) {
	state := setupProxyAuthorizationTest(t)
	const (
		securePath      = "synthetic/secure"
		interactivePath = "synthetic/interactive"
		unknownPath     = "synthetic/unknown"
		openPath        = "synthetic/open"
	)
	for _, path := range []string{securePath, interactivePath, unknownPath, openPath} {
		_, listed := proxyRouteAuthorizations[http.MethodPost+" /api/"+path]
		require.False(t, listed, "%s must not have an explicit policy entry", path)
	}
	var childReached, childSawNode bool
	sentinel := func(c *gin.Context) { c.Status(http.StatusNoContent) }
	register := func(group *gin.RouterGroup) {
		group.POST(securePath, RequireSecureSession(), RejectInDemo(), sentinel)
		group.POST(interactivePath, RequireInteractiveUser(), RequireSecureSession(), sentinel)
		group.POST(unknownPath, RequireSyntheticGuardForProxyTest(), sentinel)
		group.POST(openPath, sentinel)
	}
	child := gin.New()
	register(child.Group("/api", AuthRequired(), func(c *gin.Context) {
		childReached = true
		_, childSawNode = c.Get(nodeauth.GinPrincipalKey)
		c.Next()
	}))
	childServer := httptest.NewServer(child)
	defer childServer.Close()
	nodeID := registerProxyTestNode(t, childServer.URL)
	controller := gin.New()
	register(controller.Group("/api", AuthRequired(), Proxy()))

	tests := []struct {
		name, path, token string
		steppedUp, remote bool
		want              int
		reached           bool
	}{
		{"MFA without step-up", securePath, state.jwt, false, true, http.StatusUnauthorized, false},
		{"MFA stepped up", securePath, state.jwt, true, true, http.StatusNoContent, true},
		{"non-MFA administrator", securePath, state.plainJWT, false, true, http.StatusNoContent, true},
		{"write PAT", securePath, state.writeToken, false, true, http.StatusNoContent, true},
		{"read PAT", securePath, state.readToken, false, true, http.StatusForbidden, false},
		{"interactive MFA without step-up", interactivePath, state.jwt, false, true, http.StatusUnauthorized, false},
		{"interactive write PAT", interactivePath, state.writeToken, false, true, http.StatusForbidden, false},
		// The child keeps interactive routes to administrators and refuses the
		// node principal, which the proxy reports as 503.
		{"interactive stepped up", interactivePath, state.jwt, true, true, http.StatusServiceUnavailable, true},
		// Refused with 503, not 403: the web UI signs out on a 403 carrying
		// its token, and an unforwardable route says nothing about the session.
		{"unknown guard remote", unknownPath, state.jwt, true, true, http.StatusServiceUnavailable, false},
		{"unknown guard local", unknownPath, state.jwt, false, false, http.StatusNoContent, false},
		{"unguarded", openPath, state.jwt, false, true, http.StatusNoContent, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			childReached, childSawNode = false, false
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req := httptest.NewRequest(http.MethodPost, "/api/"+tc.path, nil).WithContext(ctx)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			if tc.remote {
				req.Header.Set("X-Node-ID", strconv.FormatUint(nodeID, 10))
			}
			if tc.steppedUp {
				req.Header.Set("X-Secure-Session-ID", state.secureSession)
			}
			res := httptest.NewRecorder()
			controller.ServeHTTP(res, req)
			require.Equal(t, tc.want, res.Code, res.Body.String())
			require.Equal(t, tc.reached, childReached)
			if tc.reached {
				require.True(t, childSawNode, "the child sees the controller as a node")
			}
			if tc.path == unknownPath && tc.remote {
				require.Contains(t, res.Body.String(), "40302")
			}
		})
	}
}

// The WebSocket proxy derives the same policy, so a guard registered after
// ProxyWs is enforced before the upgrade is forwarded.
func TestWebSocketProxyEnforcesRegisteredGuardsWithoutPolicyEntry(t *testing.T) {
	state := setupProxyAuthorizationTest(t)
	childReached := false
	child := gin.New()
	child.GET("/api/synthetic/feed", AuthRequiredWS(), func(c *gin.Context) {
		childReached = true
		c.Status(http.StatusNoContent)
	})
	childServer := httptest.NewServer(child)
	defer childServer.Close()
	nodeID := registerProxyTestNode(t, childServer.URL)
	controller := gin.New()
	controller.GET("/api/synthetic/feed", AuthRequiredWS(), ProxyWs(), RequireSecureSession(), func(c *gin.Context) { t.Error("local handler must not run") })
	controller.GET("/api/synthetic/unknown_feed", AuthRequiredWS(), ProxyWs(), RequireSyntheticGuardForProxyTest(), func(c *gin.Context) { t.Error("local handler must not run") })

	for _, tc := range []struct {
		path string
		want int
	}{
		{"/api/synthetic/feed", http.StatusUnauthorized},
		{"/api/synthetic/unknown_feed", http.StatusServiceUnavailable},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path+"?x_node_id="+strconv.FormatUint(nodeID, 10), nil)
		req.Header.Set("Authorization", "Bearer "+state.jwt)
		res := httptest.NewRecorder()
		controller.ServeHTTP(res, req)
		require.Equal(t, tc.want, res.Code, res.Body.String())
		require.False(t, childReached)
	}
}
