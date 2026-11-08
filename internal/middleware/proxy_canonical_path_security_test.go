package middleware_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"

	pluginapi "github.com/0xJacky/Nginx-UI/api/plugin"
	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/router"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"gorm.io/driver/sqlite"
)

// proxyPathNotCanonicalCode is the cosy code of the middleware refusal for a
// path the node proxy will not forward.
const proxyPathNotCanonicalCode = "40003"

const canonicalPathDNSSecret = "CANONICAL-PATH-DNS-SECRET"

// normalizingFrontProxy stands in for a reverse proxy in front of a child
// node that normalizes the path before passing it on, as nginx does with
// `proxy_pass http://127.0.0.1:9000/;` and as CDN URL normalization does: it
// decodes the path, treats a backslash as a separator like the normalizers
// that do, merges slashes and resolves dot segments. A trailing dot segment
// leaves a trailing slash, as in RFC 3986 dot segment removal.
func normalizingFrontProxy(t *testing.T, upstream string) string {
	t.Helper()
	target, err := url.Parse(upstream)
	require.NoError(t, err)
	forward := httputil.NewSingleHostReverseProxy(target)
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoded := strings.ReplaceAll(r.URL.Path, `\`, "/")
		trailingSlash := strings.HasSuffix(decoded, "/") || strings.HasSuffix(decoded, "/.") || strings.HasSuffix(decoded, "/..")
		normalized := path.Clean(decoded)
		if trailingSlash && normalized != "/" {
			normalized += "/"
		}
		r.URL.Path, r.URL.RawPath = normalized, ""
		forward.ServeHTTP(w, r)
	}))
	t.Cleanup(front.Close)
	return front.URL
}

// childRequest is what the child node saw of one forwarded request.
type childRequest struct {
	Path  string `json:"path"`
	Param string `json:"param"`
	Query string `json:"query"`
	Body  string `json:"body"`
}

// canonicalPathFixture is the relationship fixture with the plugin http
// route on the controller and a child that records every request it serves.
// The legacy node reaches the child through a normalizing front proxy, the
// paired node reaches it directly.
type canonicalPathFixture struct {
	*relationshipFixture
	server *httptest.Server

	mu   sync.Mutex
	seen []string
}

func setupCanonicalPathFixture(t *testing.T) *canonicalPathFixture {
	t.Helper()
	f := &canonicalPathFixture{relationshipFixture: setupRelationshipFixture(t, router.NodeRouters)}
	pluginapi.InitHTTPRouter(f.controller.Group("/api"))
	f.server = httptest.NewServer(f.controller)
	t.Cleanup(f.server.Close)

	cosy.RegisterModels(&model.DnsCredential{})
	cosy.InitDB(sqlite.Open(f.db.Dialector.(*sqlite.Dialector).DSN))
	require.NoError(t, f.db.Create(&model.DnsCredential{Name: "canonical-path-credential", Provider: "Cloudflare",
		ProviderCode: "cloudflare", Config: &dns.Config{Name: "Cloudflare", Code: "cloudflare",
			Configuration: &dns.Configuration{Credentials: map[string]string{"CF_DNS_API_TOKEN": canonicalPathDNSSecret}}}}).Error)

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	child := gin.New()
	child.Use(func(c *gin.Context) {
		f.mu.Lock()
		f.seen = append(f.seen, c.Request.URL.Path)
		f.mu.Unlock()
		c.Next()
	})
	mountRelationshipAPI(child, router.NodeRouters)
	// The child's plugin passthrough answers with what it received.
	child.Any("/api/plugins/:id/http/*path",
		middleware.ForWebSocketUpgrade(middleware.AuthRequiredWS(), middleware.AuthRequired()),
		func(c *gin.Context) {
			body, _ := io.ReadAll(c.Request.Body)
			seen := childRequest{Path: c.Request.URL.Path, Param: c.Param("path"), Query: c.Request.URL.RawQuery, Body: string(body)}
			if !middleware.IsWebSocketUpgrade(c.Request) {
				c.JSON(http.StatusOK, seen)
				return
			}
			conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			_ = conn.WriteJSON(seen)
		})
	// A feed the controller refuses to relay to a read-only token.
	child.GET("/api/site_navigation_ws", middleware.AuthRequiredWS(), func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("child feed"))
	})
	childServer := httptest.NewServer(child)
	t.Cleanup(childServer.Close)

	require.NoError(t, f.db.Model(&model.Node{}).Where("id = ?", f.legacyNodeID).
		Update("url", normalizingFrontProxy(t, childServer.URL)).Error)
	require.NoError(t, f.db.Model(&model.Node{}).Where("id = ?", f.pairedNodeID).
		Update("url", childServer.URL).Error)
	return f
}

func (f *canonicalPathFixture) childSeen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.seen...)
}

func (f *canonicalPathFixture) resetChild() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seen = nil
}

// send sends a request to the controller that selects nodeID. The target is
// written to the request line exactly as given.
func (f *canonicalPathFixture) send(t *testing.T, method, target, token, secureSession string, nodeID uint64, body string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if secureSession != "" {
		req.Header.Set("X-Secure-Session-ID", secureSession)
	}
	req.Header.Set("X-Node-ID", strconv.FormatUint(nodeID, 10))
	res := httptest.NewRecorder()
	f.controller.ServeHTTP(res, req)
	return res
}

// dialWebSocket opens a WebSocket through the controller the way the web UI
// does: credentials and the selected node in the query string.
func (f *canonicalPathFixture) dialWebSocket(t *testing.T, target, token string, nodeID uint64) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	query := url.Values{
		"x_node_id": {strconv.FormatUint(nodeID, 10)},
		"token":     {base64.RawURLEncoding.EncodeToString([]byte(token))},
	}
	separator := "?"
	if strings.Contains(target, "?") {
		separator = "&"
	}
	endpoint := "ws" + strings.TrimPrefix(f.server.URL, "http") + target + separator + query.Encode()
	header := http.Header{"Origin": {f.server.URL}}
	return websocket.DefaultDialer.Dial(endpoint, header)
}

func requireProxyPathRefused(t *testing.T, res *httptest.ResponseRecorder) {
	t.Helper()
	// Not 403: the web UI signs out on a 403 carrying its token.
	require.Equal(t, http.StatusBadRequest, res.Code, res.Body.String())
	require.Contains(t, res.Body.String(), proxyPathNotCanonicalCode)
}

// pairUpgradeTraversals are plugin passthrough paths that a normalizing
// proxy in front of the child resolves to the relationship endpoint.
var pairUpgradeTraversals = []string{
	"/api/plugins/x/http/../../../node/pair/upgrade",
	"/api/plugins/x/http/./../../../node/pair/upgrade",
	"/api/plugins/x/http/%2e%2e/%2e%2e/%2e%2e/node/pair/upgrade",
	"/api/plugins/x/http/%2E%2e/%2e%2E/.%2e/node/pair/upgrade",
	"/api/plugins/x/http/..%2f..%2f..%2fnode%2fpair%2fupgrade",
	"/api/plugins/x/http/..%5c..%5c..%5cnode/pair/upgrade",
}

// GHSA-2phx-h598-cv4x: the controller authorizes a request by the route it
// matched, while a proxy in front of the child may resolve the forwarded path
// to another route that the child then serves to the controller's node
// principal. A path that does not mean the same thing to every hop is refused
// before it is forwarded.
func TestProxyRefusesNonCanonicalPathsBeforeForwarding(t *testing.T) {
	f := setupCanonicalPathFixture(t)

	t.Run("relationship endpoint", func(t *testing.T) {
		before := f.controllerCredentials(t)
		for _, caller := range f.callers() {
			for _, target := range pairUpgradeTraversals {
				t.Run(caller.name+" "+target, func(t *testing.T) {
					f.resetChild()
					body := `{"controller_instance_id":"dddddddd-dddd-4ddd-8ddd-dddddddddddd","public_key":"` + newRelationshipPublicKey(t) + `"}`
					res := f.send(t, http.MethodPost, target, caller.token, caller.secureSession, f.legacyNodeID, body)
					if caller.token == f.readToken {
						// The token scope refuses every write before the proxy runs.
						require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
					} else {
						requireProxyPathRefused(t, res)
					}
					require.Empty(t, f.childSeen(), "the request reached the child node")
				})
			}
		}
		require.Equal(t, before, f.controllerCredentials(t), "the child's controller credentials changed")
	})

	t.Run("redacted read", func(t *testing.T) {
		// The direct route is relayed with the stored credentials withheld.
		res := f.send(t, http.MethodGet, "/api/dns_credentials", f.readToken, "", f.legacyNodeID, "")
		require.Equal(t, http.StatusOK, res.Code, res.Body.String())
		require.Contains(t, res.Body.String(), "canonical-path-credential")
		require.NotContains(t, res.Body.String(), canonicalPathDNSSecret)

		for _, target := range []string{
			"/api/plugins/x/http/../../../dns_credentials",
			"/api/plugins/x/http/%2e%2e/%2e%2e/%2e%2e/dns_credentials",
			"/api/plugins/x/http/..%2F..%2F..%2Fdns_credentials",
		} {
			t.Run(target, func(t *testing.T) {
				f.resetChild()
				res := f.send(t, http.MethodGet, target, f.readToken, "", f.legacyNodeID, "")
				requireProxyPathRefused(t, res)
				require.NotContains(t, res.Body.String(), canonicalPathDNSSecret)
				require.Empty(t, f.childSeen(), "the request reached the child node")
			})
		}
	})

	t.Run("websocket", func(t *testing.T) {
		// The controller refuses this child feed to a read-only token.
		conn, res, err := f.dialWebSocket(t, "/api/site_navigation_ws", f.readToken, f.legacyNodeID)
		require.Error(t, err)
		require.Nil(t, conn)
		require.NotEqual(t, http.StatusSwitchingProtocols, res.StatusCode)
		res.Body.Close()

		for _, node := range []struct {
			name string
			id   uint64
		}{
			{"legacy node behind a normalizing proxy", f.legacyNodeID},
			// The WebSocket proxy must not resolve the path itself either:
			// a node that verifies the signed path would accept the result.
			{"paired node", f.pairedNodeID},
		} {
			for _, target := range []string{
				"/api/plugins/x/http/../../../site_navigation_ws",
				"/api/plugins/x/http/%2e%2e/%2e%2e/%2e%2e/site_navigation_ws",
			} {
				t.Run(node.name+" "+target, func(t *testing.T) {
					f.resetChild()
					conn, res, err := f.dialWebSocket(t, target, f.readToken, node.id)
					if conn != nil {
						_, message, _ := conn.ReadMessage()
						conn.Close()
						t.Fatalf("the upgrade was relayed: %q", message)
					}
					require.Error(t, err)
					require.Equal(t, http.StatusBadRequest, res.StatusCode)
					body, _ := io.ReadAll(res.Body)
					res.Body.Close()
					require.Contains(t, string(body), proxyPathNotCanonicalCode)
					require.Empty(t, f.childSeen(), "the request reached the child node")
				})
			}
		}
	})
}

// Ordinary requests keep reaching the child exactly as sent: dots inside a
// segment, encoded characters other than a dot, slash or backslash, encoded
// query values, a trailing slash and WebSocket upgrades.
func TestProxyForwardsCanonicalPaths(t *testing.T) {
	f := setupCanonicalPathFixture(t)
	nodes := []struct {
		name string
		id   uint64
	}{
		{"legacy node behind a normalizing proxy", f.legacyNodeID},
		{"paired node", f.pairedNodeID},
	}

	passthrough := []struct {
		method, target, body string
		want                 childRequest
	}{
		{http.MethodGet, "/api/plugins/demo/http/items?q=a%2Fb&next=%2e%2e%2F&name=caf%C3%A9",
			"", childRequest{Path: "/api/plugins/demo/http/items", Param: "/items"}},
		{http.MethodGet, "/api/plugins/demo/http/", "", childRequest{Path: "/api/plugins/demo/http/", Param: "/"}},
		{http.MethodGet, "/api/plugins/demo/http/files/a%20b/caf%C3%A9/%E4%B8%AD%E6%96%87", "",
			childRequest{Path: "/api/plugins/demo/http/files/a b/café/中文", Param: "/files/a b/café/中文"}},
		{http.MethodGet, "/api/plugins/demo/http/a..b/.well-known/v1.2.3/...", "",
			childRequest{Path: "/api/plugins/demo/http/a..b/.well-known/v1.2.3/...", Param: "/a..b/.well-known/v1.2.3/..."}},
		{http.MethodPost, "/api/plugins/demo/http/actions/run/", `{"ok":true}`,
			childRequest{Path: "/api/plugins/demo/http/actions/run/", Param: "/actions/run/", Body: `{"ok":true}`}},
	}
	for _, node := range nodes {
		for _, tc := range passthrough {
			t.Run(node.name+" "+tc.method+" "+tc.target, func(t *testing.T) {
				res := f.send(t, tc.method, tc.target, f.writeToken, "", node.id, tc.body)
				require.Equal(t, http.StatusOK, res.Code, res.Body.String())
				var seen childRequest
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &seen))
				require.Equal(t, tc.want.Path, seen.Path)
				require.Equal(t, tc.want.Param, seen.Param)
				require.Equal(t, tc.want.Body, seen.Body)
				if strings.Contains(tc.target, "?") {
					query, err := url.ParseQuery(seen.Query)
					require.NoError(t, err)
					require.Equal(t, "a/b", query.Get("q"))
					require.Equal(t, "../", query.Get("next"))
					require.Equal(t, "café", query.Get("name"))
					require.Empty(t, query.Get("x_node_id"))
				}
			})
		}

		t.Run(node.name+" websocket", func(t *testing.T) {
			conn, res, err := f.dialWebSocket(t, "/api/plugins/demo/http/ws/feed%20one?topic=a%2Fb", f.writeToken, node.id)
			require.NoError(t, err)
			defer conn.Close()
			require.Equal(t, http.StatusSwitchingProtocols, res.StatusCode)
			var seen childRequest
			require.NoError(t, conn.ReadJSON(&seen))
			require.Equal(t, "/api/plugins/demo/http/ws/feed one", seen.Path)
			require.Equal(t, "/ws/feed one", seen.Param)
			query, err := url.ParseQuery(seen.Query)
			require.NoError(t, err)
			require.Equal(t, "a/b", query.Get("topic"))
		})

		// Site and stream names are configuration file names, so they often
		// carry dots; only a whole "." or ".." segment is a traversal.
		for _, name := range []string{"example.com.conf", "a..b", "..conf", ".hidden"} {
			t.Run(node.name+" site "+name, func(t *testing.T) {
				f.resetChild()
				res := f.send(t, http.MethodGet, "/api/sites/"+url.PathEscape(name), f.writeToken, "", node.id, "")
				require.NotContains(t, res.Body.String(), proxyPathNotCanonicalCode)
				require.Contains(t, f.childSeen(), "/api/sites/"+name)
			})
		}
	}
}

// The rule itself, through the plugin passthrough to a paired node that
// receives the path unchanged.
func TestProxyCanonicalPathRule(t *testing.T) {
	f := setupCanonicalPathFixture(t)
	const prefix = "/api/plugins/demo/http"
	for _, tc := range []struct {
		suffix  string
		refused bool
	}{
		{"/../x", true},
		{"/./x", true},
		{"/x/..", true},
		{"/x/.", true},
		{"/x/../", true},
		{"//x", true},
		{"/x//y", true},
		{"/x//", true},
		{"/%2e%2e/x", true},
		{"/%2E/x", true},
		{"/a%2eb", true},
		{"/a%2fb", true},
		{"/a%2Fb", true},
		{"/a%5cb", true},
		{"/a%5Cb", true},
		{"/a%00b", true},
		{"/a.b", false},
		{"/a..b", false},
		{"/...", false},
		{"/.x/x.", false},
		{"/x/", false},
		{"/a%20b", false},
		{"/a%25b", false},
		{"/caf%C3%A9", false},
		{"/a+b;c=d,e", false},
	} {
		t.Run(tc.suffix, func(t *testing.T) {
			f.resetChild()
			res := f.send(t, http.MethodGet, prefix+tc.suffix, f.writeToken, "", f.pairedNodeID, "")
			if tc.refused {
				requireProxyPathRefused(t, res)
				require.Empty(t, f.childSeen(), "the request reached the child node")
				return
			}
			require.Equal(t, http.StatusOK, res.Code, res.Body.String())
			require.Len(t, f.childSeen(), 1)
		})
	}
}
