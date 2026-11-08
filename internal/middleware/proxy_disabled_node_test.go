package middleware_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/require"
)

// proxyNodeDisabledCode is the cosy code of the middleware refusal for a
// request that selects a disabled node.
const proxyNodeDisabledCode = "40303"

// unreachableNode stands in for a disabled node: it counts every connection
// and request it receives, so a test can tell that nothing was sent to it.
type unreachableNode struct {
	URL         string
	connections atomic.Int32
	requests    atomic.Int32
}

func newUnreachableNode(t *testing.T) *unreachableNode {
	t.Helper()
	node := &unreachableNode{}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		node.requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			node.connections.Add(1)
		}
	}
	server.Start()
	t.Cleanup(server.Close)
	node.URL = server.URL
	return node
}

func (n *unreachableNode) requireUntouched(t *testing.T) {
	t.Helper()
	require.Zero(t, n.connections.Load(), "the proxy connected to the disabled node")
	require.Zero(t, n.requests.Load(), "the request reached the disabled node")
}

// trackedBody records whether anything read the request body.
type trackedBody struct {
	reader io.Reader
	read   atomic.Bool
}

func (b *trackedBody) Read(p []byte) (int, error) {
	b.read.Store(true)
	return b.reader.Read(p)
}

func (f *canonicalPathFixture) setNode(t *testing.T, id uint64, enabled bool, url string) {
	t.Helper()
	require.NoError(t, f.db.Model(&model.Node{}).Where("id = ?", id).
		Updates(map[string]any{"enabled": enabled, "url": url}).Error)
}

func (f *canonicalPathFixture) sendWithBody(t *testing.T, method, target, token string, nodeID uint64, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(method, target, body).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Node-ID", strconv.FormatUint(nodeID, 10))
	res := httptest.NewRecorder()
	f.controller.ServeHTTP(res, req)
	return res
}

func requireProxyNodeDisabled(t *testing.T, status int, body string) {
	t.Helper()
	// Not 403: the web UI signs out on a 403 carrying its token, and a
	// disabled node says nothing about the caller's session.
	require.NotEqual(t, http.StatusForbidden, status, body)
	require.Equal(t, http.StatusServiceUnavailable, status, body)
	require.Contains(t, body, proxyNodeDisabledCode)
}

// A disabled node is not kept up to date by the relationship maintenance job,
// so the node proxy does not forward requests to it either. The refusal comes
// before any connection to the node and before the request body is read.
func TestProxyRefusesDisabledNodes(t *testing.T) {
	f := setupCanonicalPathFixture(t)
	var childURL string
	require.NoError(t, f.db.Model(&model.Node{}).Where("id = ?", f.pairedNodeID).Pluck("url", &childURL).Error)

	nodes := []struct {
		name string
		id   uint64
	}{
		{"legacy node", f.legacyNodeID},
		{"paired node", f.pairedNodeID},
	}
	for _, node := range nodes {
		t.Run(node.name+" enabled", func(t *testing.T) {
			f.setNode(t, node.id, true, childURL)

			res := f.send(t, http.MethodPost, "/api/plugins/demo/http/actions/run", f.writeToken, "", node.id, `{"ok":true}`)
			require.Equal(t, http.StatusOK, res.Code, res.Body.String())
			require.Contains(t, res.Body.String(), `{\"ok\":true}`)

			res = f.send(t, http.MethodGet, "/api/sites/example.com.conf", f.jwt, "", node.id, "")
			require.NotContains(t, res.Body.String(), proxyNodeDisabledCode)

			conn, wsRes, err := f.dialWebSocket(t, "/api/plugins/demo/http/ws/feed", f.writeToken, node.id)
			require.NoError(t, err)
			defer conn.Close()
			require.Equal(t, http.StatusSwitchingProtocols, wsRes.StatusCode)
			var seen childRequest
			require.NoError(t, conn.ReadJSON(&seen))
			require.Equal(t, "/api/plugins/demo/http/ws/feed", seen.Path)
		})

		t.Run(node.name+" disabled", func(t *testing.T) {
			target := newUnreachableNode(t)
			f.setNode(t, node.id, false, target.URL)
			t.Cleanup(func() { f.setNode(t, node.id, true, childURL) })

			body := &trackedBody{reader: strings.NewReader(`{"ok":true}`)}
			res := f.sendWithBody(t, http.MethodPost, "/api/plugins/demo/http/actions/run", f.writeToken, node.id, body)
			requireProxyNodeDisabled(t, res.Code, res.Body.String())
			require.False(t, body.read.Load(), "the request body was read")

			// An interactive administrator must not be signed out.
			res = f.send(t, http.MethodGet, "/api/sites/example.com.conf", f.jwt, "", node.id, "")
			requireProxyNodeDisabled(t, res.Code, res.Body.String())

			conn, wsRes, err := f.dialWebSocket(t, "/api/plugins/demo/http/ws/feed", f.writeToken, node.id)
			if conn != nil {
				conn.Close()
				t.Fatal("the upgrade was relayed to the disabled node")
			}
			require.Error(t, err)
			require.NotNil(t, wsRes)
			wsBody, _ := io.ReadAll(wsRes.Body)
			wsRes.Body.Close()
			requireProxyNodeDisabled(t, wsRes.StatusCode, string(wsBody))

			target.requireUntouched(t)
		})
	}
}
