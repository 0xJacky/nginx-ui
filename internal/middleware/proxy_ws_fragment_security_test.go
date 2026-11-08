package middleware_test

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestWebSocketProxyForwardsTheAuthorizedPathWithARawHash sends WebSocket
// handshakes whose request target holds a raw '#'. Go's server keeps it as
// part of the path, so the controller routes and authorizes the full path;
// the child must receive that same path rather than the part before '#'.
func TestWebSocketProxyForwardsTheAuthorizedPathWithARawHash(t *testing.T) {
	f := setupCanonicalPathFixture(t)
	for _, target := range []string{
		"/api/plugins/demo/http/ws/feed#/other",
		"/api/plugins/demo#/http/ws/feed",
	} {
		t.Run(target, func(t *testing.T) {
			f.resetChild()
			query := url.Values{
				"x_node_id": {strconv.FormatUint(f.pairedNodeID, 10)},
				"token":     {base64.RawURLEncoding.EncodeToString([]byte(f.writeToken))},
			}
			host := strings.TrimPrefix(f.server.URL, "http://")
			conn, err := net.Dial("tcp", host)
			require.NoError(t, err)
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
			_, err = fmt.Fprintf(conn, "GET %s?%s HTTP/1.1\r\nHost: %s\r\nOrigin: %s\r\nConnection: Upgrade\r\n"+
				"Upgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n",
				target, query.Encode(), host, f.server.URL)
			require.NoError(t, err)
			res, err := http.ReadResponse(bufio.NewReader(conn), nil)
			require.NoError(t, err)
			_ = res.Body.Close()

			require.NotEmpty(t, f.childSeen(), "the handshake was not forwarded")
			for _, seen := range f.childSeen() {
				require.Equal(t, target, seen, "the child served another path than the one the controller authorized")
			}
		})
	}
}

// TestWebSocketProxyKeepsEncodedPathCharacters checks that escapes other than
// a dot, slash or backslash keep their meaning through the WebSocket proxy.
func TestWebSocketProxyKeepsEncodedPathCharacters(t *testing.T) {
	f := setupCanonicalPathFixture(t)
	for raw, want := range map[string]string{
		"/api/plugins/demo/http/a%2541b": "/a%41b",
		"/api/plugins/demo/http/a%3Fb":   "/a?b",
		"/api/plugins/demo/http/a%23b":   "/a#b",
		"/api/plugins/demo/http/a%25b":   "/a%b",
		"/api/plugins/demo/http/a;b,c":   "/a;b,c",
	} {
		t.Run(raw, func(t *testing.T) {
			conn, _, err := f.dialWebSocket(t, raw, f.writeToken, f.pairedNodeID)
			require.NoError(t, err)
			defer conn.Close()
			var seen childRequest
			require.NoError(t, conn.ReadJSON(&seen))
			require.Equal(t, want, seen.Param)
		})
	}
}
