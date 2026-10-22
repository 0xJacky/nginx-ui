package middleware

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestUpgradeWebSocketProxyRequiresSecureSessionBeforeReachingNode(t *testing.T) {
	state := setupProxyAuthorizationTest(t)
	var reached atomic.Int32
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	child := gin.New()
	child.GET("/api/upgrade/perform", AuthRequiredWS(), RequireSecureSession(), func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		reached.Add(1)
		var control map[string]any
		if err := conn.ReadJSON(&control); err != nil {
			t.Errorf("read upgrade control: %v", err)
			return
		}
		if err := conn.WriteJSON(control); err != nil {
			t.Errorf("echo upgrade control: %v", err)
		}
	})
	childServer := httptest.NewServer(child)
	defer childServer.Close()
	nodeID := registerProxyTestNode(t, childServer.URL)
	controller := gin.New()
	controller.GET("/api/upgrade/perform", AuthRequiredWS(), ProxyWs(), RequireSecureSession(), func(c *gin.Context) {
		t.Error("upgrade must reach the selected node, not the local handler")
	})
	server := httptest.NewServer(controller)
	defer server.Close()

	for _, tc := range []struct {
		name, token, secureSession string
		want                       int
	}{
		{"missing session", state.jwt, "", http.StatusUnauthorized},
		{"invalid session", state.jwt, "invalid", http.StatusUnauthorized},
		{"verified session", state.jwt, state.secureSession, http.StatusSwitchingProtocols},
		{"no MFA enrolled", state.plainJWT, "", http.StatusSwitchingProtocols},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := reached.Load()
			values := url.Values{
				"x_node_id":           {strconv.FormatUint(nodeID, 10)},
				"token":               {base64.RawURLEncoding.EncodeToString([]byte(tc.token))},
				"X-Secure-Session-ID": {tc.secureSession},
			}
			endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/upgrade/perform?" + values.Encode()
			conn, response, err := websocket.DefaultDialer.Dial(endpoint, nil)
			require.NotNil(t, response)
			require.Equal(t, tc.want, response.StatusCode)
			if tc.want != http.StatusSwitchingProtocols {
				require.Error(t, err)
				require.Nil(t, conn)
				response.Body.Close()
				require.Equal(t, before, reached.Load())
				return
			}
			require.NoError(t, err)
			defer conn.Close()
			control := map[string]any{"channel": "stable", "dry_run": true}
			require.NoError(t, conn.WriteJSON(control))
			var received map[string]any
			require.NoError(t, conn.ReadJSON(&received))
			require.Equal(t, control, received)
			require.Equal(t, before+1, reached.Load())
		})
	}
}
