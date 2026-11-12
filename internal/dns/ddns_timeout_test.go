package dns

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newHangingIPServer returns a server that never answers until the test ends,
// standing in for a public IP endpoint that accepts the connection and stalls.
func newHangingIPServer(t *testing.T) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	// Cleanups run last-in first-out: unblock the handlers, then close.
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	return server
}

// A hung endpoint must not use up the time the fallbacks after it need (#2104).
func TestResolvePublicIPsFallsBackPastHungEndpoint(t *testing.T) {
	hung := newHangingIPServer(t)
	working := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("2001:db8::20"))
	}))
	t.Cleanup(working.Close)

	restore := OverrideIPEndpointsForTest([]string{"http://127.0.0.1:1"}, []string{hung.URL, working.URL})
	t.Cleanup(restore)

	start := time.Now()
	snapshot, err := resolvePublicIPs(context.Background(), DDNSIPVersionIPv6)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Equal(t, "2001:db8::20", snapshot.IPv6)
	// One attempt timeout for the hung endpoint plus a fast answer; the old
	// shared 8s budget never reached the fallback at all.
	require.Less(t, elapsed, 7*time.Second)
}
