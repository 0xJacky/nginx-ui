package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// TestWebSocketProxyVerifiesChildCertificate points both nodes at an https
// child with a self-signed certificate. The HTTP node proxy verifies the
// certificate unless [http] InsecureSkipVerify is enabled; the WebSocket
// node proxy must follow the same setting instead of skipping verification.
func TestWebSocketProxyVerifiesChildCertificate(t *testing.T) {
	f := setupProxyCredentialFixture(t, (*recordingChild).handler)
	tlsChild := httptest.NewTLSServer(f.child.handler())
	t.Cleanup(tlsChild.Close)
	require.NoError(t, f.db.Model(&model.Node{}).Where("id IN ?", []uint64{f.legacyNodeID, f.pairedNodeID}).
		Update("url", tlsChild.URL).Error)

	original := settings.HTTPSettings.InsecureSkipVerify
	t.Cleanup(func() { settings.HTTPSettings.InsecureSkipVerify = original })

	dial := func(t *testing.T, nodeID uint64) (*http.Response, error) {
		t.Helper()
		query := url.Values{"token": {f.shortToken}, "x_node_id": {strconv.FormatUint(nodeID, 10)}}
		endpoint := "ws" + strings.TrimPrefix(f.server.URL, "http") + "/api/analytic?" + query.Encode()
		conn, res, err := websocket.DefaultDialer.Dial(endpoint, http.Header{"Origin": {f.server.URL}})
		if conn != nil {
			_ = conn.Close()
		}
		return res, err
	}

	for _, node := range []struct {
		name string
		id   uint64
	}{{"paired", f.pairedNodeID}, {"legacy", f.legacyNodeID}} {
		t.Run(node.name+" refused by default", func(t *testing.T) {
			settings.HTTPSettings.InsecureSkipVerify = false
			before := len(f.child.all())

			_, err := dial(t, node.id)
			require.Error(t, err, "the handshake reached a child whose certificate is not trusted")
			require.Len(t, f.child.all(), before, "the handshake reached a child whose certificate is not trusted")

			// The HTTP node proxy refuses the same child.
			res := f.get(t, "/api/analytic/init", node.id)
			require.NotEqual(t, http.StatusOK, res.Code, res.Body.String())
			require.Len(t, f.child.all(), before)
		})

		t.Run(node.name+" connects with InsecureSkipVerify", func(t *testing.T) {
			settings.HTTPSettings.InsecureSkipVerify = true

			res, err := dial(t, node.id)
			require.NoError(t, err)
			require.Equal(t, http.StatusSwitchingProtocols, res.StatusCode)
			seen := f.child.last(t)
			require.Equal(t, "/api/analytic", seen.Path)
			requireNodeCredential(t, f, node.id, seen)
		})
	}
}
