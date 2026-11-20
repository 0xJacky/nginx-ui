package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	pluginapi "github.com/0xJacky/Nginx-UI/api/plugin"
	"github.com/0xJacky/Nginx-UI/api/terminal"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/router"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// forwardedRequest is what a child node received from the controller's
// cluster proxy.
type forwardedRequest struct {
	Path   string
	Query  url.Values
	Header http.Header
	// AsNode reports whether the child authenticated the request as the
	// controller's node principal. It is only set by authenticatingChild.
	AsNode bool
}

// recordingChild stands in for a child node: it accepts every request and
// keeps what arrived, so a test can check what the controller forwarded.
type recordingChild struct {
	mu   sync.Mutex
	seen []forwardedRequest
}

func (r *recordingChild) record(req *http.Request, asNode bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, forwardedRequest{Path: req.URL.Path, Query: req.URL.Query(), Header: req.Header.Clone(), AsNode: asNode})
}

func (r *recordingChild) last(t *testing.T) forwardedRequest {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.NotEmpty(t, r.seen, "the request did not reach the child")
	return r.seen[len(r.seen)-1]
}

func (r *recordingChild) all() []forwardedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]forwardedRequest(nil), r.seen...)
}

var forwardedUpgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

func answerForwarded(w http.ResponseWriter, req *http.Request) {
	if middleware.IsWebSocketUpgrade(req) {
		conn, err := forwardedUpgrader.Upgrade(w, req, nil)
		if err == nil {
			_ = conn.Close()
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{}`))
}

func (r *recordingChild) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.record(req, false)
		answerForwarded(w, req)
	})
}

// authenticatingChild is a child node that runs the real node authentication
// in front of the routes the controller forwards to, the way a child mounts
// them, and records only requests it authenticated.
func (r *recordingChild) authenticatingChild() http.Handler {
	engine := gin.New()
	serve := func(c *gin.Context) {
		_, asNode := c.Get(nodeauth.GinPrincipalKey)
		r.record(c.Request, asNode)
		answerForwarded(c.Writer, c.Request)
	}
	engine.GET("/api/host/setup/publickey", middleware.AuthRequired(), serve)
	engine.GET("/api/analytic", middleware.AuthRequiredWS(), serve)
	engine.GET("/api/pty", middleware.AuthRequiredWS(), serve)
	engine.Any("/api/plugins/:id/http/*path",
		middleware.ForWebSocketUpgrade(middleware.AuthRequiredWS(), middleware.AuthRequired()), serve)
	return engine
}

const (
	forwardedBindingCookie = "_nginx_ui_secure_session_http"
	forwardedBindingValue  = "0123456789abcdef0123456789abcdef"
)

// proxyCredentialFixture is a controller whose two nodes, one paired and one
// on the legacy shared secret, both point at child.
type proxyCredentialFixture struct {
	*relationshipFixture
	child      *recordingChild
	server     *httptest.Server
	shortToken string
	cookie     string
}

func setupProxyCredentialFixture(t *testing.T, childHandler func(*recordingChild) http.Handler) *proxyCredentialFixture {
	t.Helper()
	f := &proxyCredentialFixture{relationshipFixture: setupRelationshipFixture(t, nil), child: &recordingChild{}}
	childServer := httptest.NewServer(childHandler(f.child))
	t.Cleanup(childServer.Close)
	require.NoError(t, f.db.Model(&model.Node{}).Where("id IN ?", []uint64{f.legacyNodeID, f.pairedNodeID}).
		Update("url", childServer.URL).Error)

	// Mount the WebSocket routers the way router.InitRouter does.
	api := f.controller.Group("/api")
	terminal.InitRouter(api.Group("/", middleware.AuthRequiredWS()))
	ws := api.Group("/", middleware.AuthRequiredWS(), middleware.ProxyWs())
	for _, register := range router.ProxiedWebSocketRouters {
		register(ws)
	}
	// Plugin passthrough forwards through the same node proxies.
	pluginapi.InitHTTPRouter(api)
	f.server = httptest.NewServer(f.controller)
	t.Cleanup(f.server.Close)

	var admin model.User
	require.NoError(t, f.db.Where("name = ?", "relationship-mfa-admin").Take(&admin).Error)
	var err error
	f.shortToken, err = internaluser.GenerateShortTokenForSession(admin.ID, f.jwt)
	require.NoError(t, err)
	f.cookie = "secure_session_id=" + f.secureSession + "; " +
		forwardedBindingCookie + "=" + forwardedBindingValue + "; theme=dark"
	return f
}

// get sends what the web UI sends for an HTTP request with a node selected.
func (f *proxyCredentialFixture) get(t *testing.T, path string, nodeID uint64) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	req.Header.Set("Authorization", f.jwt)
	req.Header.Set("Proxy-Authorization", "Basic "+f.jwt)
	req.Header.Set("X-Secure-Session-ID", f.secureSession)
	req.Header.Set("X-Node-ID", strconv.FormatUint(nodeID, 10))
	req.Header.Set("Cookie", f.cookie)
	res := httptest.NewRecorder()
	f.controller.ServeHTTP(res, req)
	return res
}

// dial opens a WebSocket the way the web UI does with a node selected.
func (f *proxyCredentialFixture) dial(t *testing.T, path string, query url.Values) {
	t.Helper()
	endpoint := "ws" + strings.TrimPrefix(f.server.URL, "http") + path + "?" + query.Encode()
	conn, res, err := websocket.DefaultDialer.Dial(endpoint, http.Header{
		"Origin": {f.server.URL},
		"Cookie": {f.cookie},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, res.StatusCode)
	_ = conn.Close()
}

// requireNoCallerCredentials checks that a forwarded request carries none of
// the credentials the caller authenticated to the controller with.
func requireNoCallerCredentials(t *testing.T, seen forwardedRequest) {
	t.Helper()
	for _, name := range []string{"Authorization", "Proxy-Authorization", "Cookie", "X-Secure-Session-ID", "X-Node-ID"} {
		require.Empty(t, seen.Header.Values(name), "%s reached the child", name)
	}
	for _, key := range []string{"token", "X-Secure-Session-ID", "x_node_id"} {
		require.NotContains(t, seen.Query, key, "the %s query parameter reached the child", key)
	}
}

func requireNodeCredential(t *testing.T, f *proxyCredentialFixture, nodeID uint64, seen forwardedRequest) {
	t.Helper()
	if nodeID == f.pairedNodeID {
		require.NotEmpty(t, seen.Header.Get("Signature"), "the controller signs as the node")
		require.NotEmpty(t, seen.Header.Get("Signature-Input"))
	} else {
		require.NotEmpty(t, seen.Header.Get("X-Node-Secret"), "the controller sends the legacy secret")
	}
}

// TestClusterProxyDoesNotForwardCallerCredentialsToChild drives the real HTTP
// and WebSocket node proxies with what the web UI sends and checks that the
// child receives only the controller's node credential, never the caller's
// session token, short token, secure session or cookies.
func TestClusterProxyDoesNotForwardCallerCredentialsToChild(t *testing.T) {
	f := setupProxyCredentialFixture(t, (*recordingChild).handler)

	nodes := []struct {
		name string
		id   uint64
	}{{"paired", f.pairedNodeID}, {"legacy", f.legacyNodeID}}

	for _, node := range nodes {
		t.Run("http "+node.name, func(t *testing.T) {
			res := f.get(t, "/api/analytic/init", node.id)
			require.Equal(t, http.StatusOK, res.Code, res.Body.String())

			seen := f.child.last(t)
			requireNoCallerCredentials(t, seen)
			requireNodeCredential(t, f, node.id, seen)
		})

		t.Run("websocket "+node.name, func(t *testing.T) {
			f.dial(t, "/api/analytic", url.Values{
				"token":     {f.shortToken},
				"x_node_id": {strconv.FormatUint(node.id, 10)},
			})

			seen := f.child.last(t)
			require.Equal(t, "/api/analytic", seen.Path)
			requireNoCallerCredentials(t, seen)
			requireNodeCredential(t, f, node.id, seen)
		})
		t.Run("plugin http "+node.name, func(t *testing.T) {
			res := f.get(t, "/api/plugins/demo/http/feed", node.id)
			require.Equal(t, http.StatusOK, res.Code, res.Body.String())

			seen := f.child.last(t)
			require.Equal(t, "/api/plugins/demo/http/feed", seen.Path)
			requireNoCallerCredentials(t, seen)
			requireNodeCredential(t, f, node.id, seen)
		})

		t.Run("plugin websocket "+node.name, func(t *testing.T) {
			f.dial(t, "/api/plugins/demo/http/feed", url.Values{
				"token":     {f.shortToken},
				"x_node_id": {strconv.FormatUint(node.id, 10)},
			})

			seen := f.child.last(t)
			require.Equal(t, "/api/plugins/demo/http/feed", seen.Path)
			requireNoCallerCredentials(t, seen)
			requireNodeCredential(t, f, node.id, seen)
		})
	}

	t.Run("terminal websocket", func(t *testing.T) {
		f.dial(t, "/api/pty", url.Values{
			"X-Secure-Session-ID": {f.secureSession},
			"token":               {f.shortToken},
			"x_node_id":           {strconv.FormatUint(f.pairedNodeID, 10)},
		})

		seen := f.child.last(t)
		require.Equal(t, "/api/pty", seen.Path)
		requireNoCallerCredentials(t, seen)
		requireNodeCredential(t, f, f.pairedNodeID, seen)
	})

	// Nothing any child recorded can be replayed against the controller as
	// the administrator: no request carried a caller credential.
	t.Run("nothing replayable reached the child", func(t *testing.T) {
		recorded := f.child.all()
		require.NotEmpty(t, recorded)
		for _, seen := range recorded {
			require.Empty(t, seen.Header.Values("Authorization"), seen.Path)
			require.Empty(t, seen.Header.Values("X-Secure-Session-ID"), seen.Path)
			require.Empty(t, seen.Header.Values("Cookie"), seen.Path)
			require.Empty(t, seen.Query["token"], seen.Path)
			require.Empty(t, seen.Query["X-Secure-Session-ID"], seen.Path)
			for _, values := range seen.Header {
				for _, value := range values {
					require.NotContains(t, value, f.jwt, seen.Path)
					require.NotContains(t, value, f.secureSession, seen.Path)
					require.NotContains(t, value, f.shortToken, seen.Path)
				}
			}
			for _, values := range seen.Query {
				for _, value := range values {
					require.NotContains(t, value, f.jwt, seen.Path)
					require.NotContains(t, value, f.secureSession, seen.Path)
					require.NotContains(t, value, f.shortToken, seen.Path)
				}
			}
		}
	})
}

// TestClusterProxyChildAuthenticatesForwardedRequestsAsNode is the legitimate
// side of the test above: with the caller's credentials removed, a child that
// runs the real node authentication still accepts every forwarded request as
// the controller's node, and the verified two-factor marker the controller
// signs still reaches the one route that reads it.
func TestClusterProxyChildAuthenticatesForwardedRequestsAsNode(t *testing.T) {
	f := setupProxyCredentialFixture(t, (*recordingChild).authenticatingChild)

	nodes := []struct {
		name string
		id   uint64
	}{{"paired", f.pairedNodeID}, {"legacy", f.legacyNodeID}}

	for _, node := range nodes {
		t.Run("http "+node.name, func(t *testing.T) {
			res := f.get(t, "/api/host/setup/publickey", node.id)
			require.Equal(t, http.StatusOK, res.Code, res.Body.String())

			seen := f.child.last(t)
			require.True(t, seen.AsNode, "the child did not authenticate the request as the node")
			require.Equal(t, []string{"true"}, seen.Query["__nginx_ui_proxy_verified_two_factor"],
				"the stepped-up caller's verified two-factor marker must reach the child")
			requireNoCallerCredentials(t, seen)
		})

		t.Run("websocket "+node.name, func(t *testing.T) {
			f.dial(t, "/api/analytic", url.Values{
				"token":     {f.shortToken},
				"x_node_id": {strconv.FormatUint(node.id, 10)},
			})

			seen := f.child.last(t)
			require.Equal(t, "/api/analytic", seen.Path)
			require.True(t, seen.AsNode, "the child did not authenticate the handshake as the node")
			requireNoCallerCredentials(t, seen)
		})

		t.Run("terminal websocket "+node.name, func(t *testing.T) {
			f.dial(t, "/api/pty", url.Values{
				"X-Secure-Session-ID": {f.secureSession},
				"token":               {f.shortToken},
				"x_node_id":           {strconv.FormatUint(node.id, 10)},
			})

			seen := f.child.last(t)
			require.Equal(t, "/api/pty", seen.Path)
			require.True(t, seen.AsNode, "the child did not authenticate the handshake as the node")
			requireNoCallerCredentials(t, seen)
		})
		t.Run("plugin http "+node.name, func(t *testing.T) {
			res := f.get(t, "/api/plugins/demo/http/feed", node.id)
			require.Equal(t, http.StatusOK, res.Code, res.Body.String())

			seen := f.child.last(t)
			require.True(t, seen.AsNode, "the child did not authenticate the request as the node")
			requireNoCallerCredentials(t, seen)
		})

		t.Run("plugin websocket "+node.name, func(t *testing.T) {
			f.dial(t, "/api/plugins/demo/http/feed", url.Values{
				"token":     {f.shortToken},
				"x_node_id": {strconv.FormatUint(node.id, 10)},
			})

			seen := f.child.last(t)
			require.True(t, seen.AsNode, "the child did not authenticate the handshake as the node")
			requireNoCallerCredentials(t, seen)
		})
	}
}
