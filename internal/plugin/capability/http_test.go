package capability

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// fakeHTTPHost is a HTTPHost whose answers the test controls. It is a
// separate type from dns01_test.go's fakeHost, which serves a different
// interface.
type fakeHTTPHost struct {
	info     *plugin.Info
	infoErr  error
	manifest *protocol.Manifest
	dataDir  string

	caller     jsonrpc.Caller
	acquireErr error
	initResult protocol.InitializeResult
	hasInit    bool

	mu       sync.Mutex
	acquired []string
	released int
}

func (h *fakeHTTPHost) Get(string) (*plugin.Info, error) { return h.info, h.infoErr }

func (h *fakeHTTPHost) Manifest(string) (*protocol.Manifest, bool) {
	return h.manifest, h.manifest != nil
}

func (h *fakeHTTPHost) Acquire(_ context.Context, id string) (jsonrpc.Caller, func(), error) {
	h.mu.Lock()
	h.acquired = append(h.acquired, id)
	h.mu.Unlock()
	release := func() {
		h.mu.Lock()
		h.released++
		h.mu.Unlock()
	}
	if h.acquireErr != nil {
		return nil, release, h.acquireErr
	}
	return h.caller, release, nil
}

func (h *fakeHTTPHost) DataDir(string) string { return h.dataDir }

func (h *fakeHTTPHost) InitializeResult(string) (protocol.InitializeResult, bool) {
	return h.initResult, h.hasInit
}

func (h *fakeHTTPHost) acquireCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.acquired)
}

func (h *fakeHTTPHost) releaseCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.released
}

// httpManifest builds a minimal manifest declaring the http capability.
func httpManifest(listen string) *protocol.Manifest {
	return &protocol.Manifest{
		ID:           "official.http",
		Name:         "HTTP Plugin",
		Version:      "1.0.0",
		APIVersion:   protocol.APIVersion,
		Capabilities: []string{protocol.CapabilityHTTP},
		HTTP:         &protocol.ManifestHTTP{Listen: listen},
	}
}

func enabledInfo() *plugin.Info {
	return &plugin.Info{
		ID:           "official.http",
		Enabled:      true,
		Capabilities: []string{protocol.CapabilityHTTP},
	}
}

// newHTTPTestContext builds a gin.Context the way AuthRequired leaves it: the
// authenticated user set under "user" and the route params a real router
// would have parsed from "/plugins/:id/http/*path".
func newHTTPTestContext(method, target string, body io.Reader, id, subPath string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, body)
	c.Params = gin.Params{{Key: "id", Value: id}, {Key: "path", Value: subPath}}
	c.Set("user", &model.User{Model: model.Model{ID: 7}, Name: "alice"})
	return c, recorder
}

func TestNewHTTPHandlerRejectsUnknownOrUndeclaredPlugins(t *testing.T) {
	cases := []struct {
		name string
		host *fakeHTTPHost
		id   string
	}{
		{"invalid id", &fakeHTTPHost{}, "not an id"},
		{"unknown plugin", &fakeHTTPHost{infoErr: plugin.ErrPluginNotFound}, "official.http"},
		{"disabled", &fakeHTTPHost{info: &plugin.Info{ID: "official.http", Enabled: false, Capabilities: []string{protocol.CapabilityHTTP}}}, "official.http"},
		{"capability not declared", &fakeHTTPHost{info: &plugin.Info{ID: "official.http", Enabled: true}}, "official.http"},
		{
			"manifest missing http block",
			&fakeHTTPHost{info: enabledInfo(), manifest: &protocol.Manifest{ID: "official.http", Capabilities: []string{protocol.CapabilityHTTP}}},
			"official.http",
		},
		{
			"unknown listen mode",
			&fakeHTTPHost{info: enabledInfo(), manifest: httpManifest("carrier-pigeon")},
			"official.http",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := newHTTPTestContext(http.MethodGet, "/api/plugins/x/http/echo", nil, tc.id, "/echo")
			NewHTTPHandler(tc.host)(c)
			assert.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}

// startUnixHTTPServer runs handler behind a real unix socket in dir, the same
// layout the http capability expects: "<DataDir>/http.sock".
func startUnixHTTPServer(t *testing.T, dir string, handler http.HandlerFunc) {
	t.Helper()
	listener, err := net.Listen("unix", filepath.Join(dir, httpSocketName))
	require.NoError(t, err)

	server := &http.Server{Handler: handler}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
}

// newUnixProxyServer wraps NewHTTPHandler in a real HTTP server. The unix
// mode goes through httputil.ReverseProxy, which probes the ResponseWriter
// for http.CloseNotifier; httptest.ResponseRecorder does not implement it, so
// the handler must be exercised through a real listener instead of being
// called with a bare recorder.
func newUnixProxyServer(t *testing.T, host HTTPHost) *httptest.Server {
	t.Helper()
	router := gin.New()
	router.Any("/plugins/:id/http/*path", func(c *gin.Context) {
		c.Set("user", &model.User{Model: model.Model{ID: 7}, Name: "alice"})
		NewHTTPHandler(host)(c)
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}

func TestNewHTTPHandlerProxiesToTheUnixSocket(t *testing.T) {
	dir := t.TempDir()

	var (
		mu           sync.Mutex
		sawMethod    string
		sawPath      string
		sawQuery     string
		sawUser      string
		sawUserID    string
		sawAuth      string
		sawCookie    string
		sawHasCookie bool
	)
	startUnixHTTPServer(t, dir, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		mu.Lock()
		sawMethod = r.Method
		sawPath = r.URL.Path
		sawQuery = r.URL.RawQuery
		sawUser = r.Header.Get(headerPluginUser)
		sawUserID = r.Header.Get(headerPluginUserID)
		sawAuth = r.Header.Get("Authorization")
		_, sawHasCookie = r.Header["Cookie"]
		sawCookie = r.Header.Get("Cookie")
		mu.Unlock()

		w.Header().Set("X-Plugin-Reply", "yes")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(append([]byte("echo:"), body...))
	})

	host := &fakeHTTPHost{info: enabledInfo(), manifest: httpManifest("unix"), dataDir: dir}
	server := newUnixProxyServer(t, host)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/plugins/official.http/http/hello?x=1", strings.NewReader("payload"))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Cookie", "session=abc")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "yes", resp.Header.Get("X-Plugin-Reply"))
	assert.Equal(t, "echo:payload", string(respBody))

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, http.MethodPost, sawMethod)
	assert.Equal(t, "/hello", sawPath)
	assert.Equal(t, "x=1", sawQuery)
	assert.Equal(t, "alice", sawUser)
	assert.Equal(t, "7", sawUserID)
	assert.Empty(t, sawAuth, "Authorization must not reach the plugin")
	assert.False(t, sawHasCookie, "Cookie must not reach the plugin")
	assert.Empty(t, sawCookie)

	assert.Equal(t, 1, host.acquireCount())
	assert.Equal(t, 1, host.releaseCount())
}

func TestNewHTTPHandlerUnixProxyReturns503WhenTheSocketIsMissing(t *testing.T) {
	dir := t.TempDir() // no listener bound here

	host := &fakeHTTPHost{info: enabledInfo(), manifest: httpManifest("unix"), dataDir: dir}
	server := newUnixProxyServer(t, host)

	resp, err := http.Get(server.URL + "/plugins/official.http/http/x")
	require.NoError(t, err)
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Contains(t, string(respBody), "55004")
	assert.Equal(t, 1, host.releaseCount())
}

func TestNewHTTPHandlerUnixProxyReturns503WhenAcquireFails(t *testing.T) {
	host := &fakeHTTPHost{info: enabledInfo(), manifest: httpManifest("unix"), acquireErr: plugin.ErrPluginNotRunning}
	server := newUnixProxyServer(t, host)

	resp, err := http.Get(server.URL + "/plugins/official.http/http/x")
	require.NoError(t, err)
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Contains(t, string(respBody), "55004")
	assert.Equal(t, 1, host.releaseCount())
	assert.Equal(t, 1, host.acquireCount())
}

func TestNewHTTPHandlerRPCRoundTrip(t *testing.T) {
	caller := newFakeCaller()
	caller.results[protocol.MethodHTTPHandle] = protocol.HTTPHandleResult{
		Status:     http.StatusTeapot,
		Headers:    map[string][]string{"X-Reply": {"brewed"}},
		BodyBase64: base64.StdEncoding.EncodeToString([]byte("kettle")),
	}
	host := &fakeHTTPHost{info: enabledInfo(), manifest: httpManifest("rpc"), caller: caller}

	c, recorder := newHTTPTestContext(http.MethodPost, "/api/plugins/official.http/http/brew?x=1",
		strings.NewReader("water"), "official.http", "/brew")
	c.Request.Header.Set("Authorization", "Bearer secret")

	NewHTTPHandler(host)(c)

	require.Equal(t, http.StatusTeapot, recorder.Code)
	assert.Equal(t, "brewed", recorder.Header().Get("X-Reply"))
	assert.Equal(t, "kettle", recorder.Body.String())

	calls := caller.methodCalls(protocol.MethodHTTPHandle)
	require.Len(t, calls, 1)
	var params protocol.HTTPHandleParams
	require.NoError(t, json.Unmarshal(calls[0].Params, &params))
	assert.Equal(t, http.MethodPost, params.Method)
	assert.Equal(t, "/brew", params.Path)
	assert.Equal(t, "x=1", params.Query)
	assert.Equal(t, "7", params.User.ID)
	assert.Equal(t, "alice", params.User.Name)
	decoded, err := base64.StdEncoding.DecodeString(params.BodyBase64)
	require.NoError(t, err)
	assert.Equal(t, "water", string(decoded))
	_, hasAuth := params.Headers["Authorization"]
	assert.False(t, hasAuth, "Authorization must not be forwarded to the plugin")

	assert.Equal(t, 1, host.releaseCount())
}

func TestNewHTTPHandlerRPCRejectsOversizedBody(t *testing.T) {
	caller := newFakeCaller()
	host := &fakeHTTPHost{info: enabledInfo(), manifest: httpManifest("rpc"), caller: caller}

	oversized := bytes.Repeat([]byte("a"), maxRPCBodyBytes+1)
	c, recorder := newHTTPTestContext(http.MethodPost, "/api/plugins/official.http/http/big",
		bytes.NewReader(oversized), "official.http", "/big")

	NewHTTPHandler(host)(c)

	assert.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	assert.Empty(t, caller.methodCalls(protocol.MethodHTTPHandle))
	assert.Zero(t, host.acquireCount())
}

func TestNewHTTPHandlerRPCReturns503OnCallError(t *testing.T) {
	caller := newFakeCaller()
	caller.errs[protocol.MethodHTTPHandle] = &protocol.Error{Code: protocol.CodeInternalError, Message: "boom"}
	host := &fakeHTTPHost{info: enabledInfo(), manifest: httpManifest("rpc"), caller: caller}

	c, recorder := newHTTPTestContext(http.MethodGet, "/api/plugins/official.http/http/x", nil, "official.http", "/x")

	NewHTTPHandler(host)(c)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "boom")
	assert.Equal(t, 1, host.releaseCount())
}

func TestNewHTTPHandlerProxiesWebSocketsWithoutHandshakeCredentials(t *testing.T) {
	dir := t.TempDir()

	queries := make(chan string, 1)
	upgrader := websocket.Upgrader{}
	startUnixHTTPServer(t, dir, func(w http.ResponseWriter, r *http.Request) {
		queries <- r.URL.RawQuery
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			kind, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err = conn.WriteMessage(kind, append([]byte("echo:"), data...)); err != nil {
				return
			}
		}
	})

	host := &fakeHTTPHost{info: enabledInfo(), manifest: httpManifest("unix"), dataDir: dir}
	server := newUnixProxyServer(t, host)

	target := "ws" + strings.TrimPrefix(server.URL, "http") + "/plugins/official.http/http/events?token=secret&x_node_id=3&room=1"
	conn, _, err := websocket.DefaultDialer.Dial(target, nil)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("ping")))
	_, reply, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "echo:ping", string(reply))

	assert.Equal(t, "room=1", <-queries, "session credentials must not reach the plugin")
}

func TestNewHTTPHandlerKeepsQueryOfPlainRequests(t *testing.T) {
	dir := t.TempDir()

	queries := make(chan string, 1)
	startUnixHTTPServer(t, dir, func(w http.ResponseWriter, r *http.Request) {
		queries <- r.URL.RawQuery
	})

	host := &fakeHTTPHost{info: enabledInfo(), manifest: httpManifest("unix"), dataDir: dir}
	server := newUnixProxyServer(t, host)

	resp, err := http.Get(server.URL + "/plugins/official.http/http/x?token=mine&x_node_id=3")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "token=mine&x_node_id=3", <-queries)
}
