package dns

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// newIPServerOn starts a server on the given listener address that answers
// every request with body. It reports false when the address cannot be bound.
func newIPServerOn(t *testing.T, addr, body string) (*httptest.Server, bool) {
	t.Helper()
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, false
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return server, true
}

// The servers that must stay unreachable answer with an address of the family
// being probed, so only the dial, not the body family check, can reject them.
func TestIPProbeClientsArePinnedToAddressFamily(t *testing.T) {
	t.Run("ipv6 probe cannot reach an ipv4-only server", func(t *testing.T) {
		server, ok := newIPServerOn(t, "127.0.0.1:0", "2001:db8::20")
		require.True(t, ok)
		_, err := fetchIP(context.Background(), server.URL, ipFamilyV6)
		require.Error(t, err)
	})

	t.Run("ipv4 probe reaches an ipv4 server", func(t *testing.T) {
		server, ok := newIPServerOn(t, "127.0.0.1:0", "198.51.100.12")
		require.True(t, ok)
		ip, err := fetchIP(context.Background(), server.URL, ipFamilyV4)
		require.NoError(t, err)
		require.Equal(t, "198.51.100.12", ip)
	})

	t.Run("ipv4 probe cannot reach an ipv6-only server", func(t *testing.T) {
		server, ok := newIPServerOn(t, "[::1]:0", "198.51.100.12")
		if !ok {
			t.Skip("IPv6 loopback is unavailable")
		}
		_, err := fetchIP(context.Background(), server.URL, ipFamilyV4)
		require.Error(t, err)
	})

	t.Run("ipv6 probe reaches an ipv6 server", func(t *testing.T) {
		server, ok := newIPServerOn(t, "[::1]:0", "2001:db8::20")
		if !ok {
			t.Skip("IPv6 loopback is unavailable")
		}
		ip, err := fetchIP(context.Background(), server.URL, ipFamilyV6)
		require.NoError(t, err)
		require.Equal(t, "2001:db8::20", ip)
	})
}

// A dual-stack host name must be probed over the family being detected, the
// way api64.ipify.org answers over whichever family the client connects with.
func TestIPProbeOfDualStackNameUsesRequestedFamily(t *testing.T) {
	ipv6Addrs, err := net.DefaultResolver.LookupIP(context.Background(), "ip6", "localhost")
	if err != nil || len(ipv6Addrs) == 0 {
		t.Skip("localhost does not resolve to an IPv6 address")
	}
	ipv4Addrs, err := net.DefaultResolver.LookupIP(context.Background(), "ip4", "localhost")
	if err != nil || len(ipv4Addrs) == 0 {
		t.Skip("localhost does not resolve to an IPv4 address")
	}

	ipv4Server, ok := newIPServerOn(t, "127.0.0.1:0", "198.51.100.12")
	require.True(t, ok)
	port := strconv.Itoa(ipv4Server.Listener.Addr().(*net.TCPAddr).Port)
	if _, ok := newIPServerOn(t, net.JoinHostPort("::1", port), "2001:db8::20"); !ok {
		t.Skip("cannot bind the same port on IPv6 loopback")
	}

	endpoint := "http://" + net.JoinHostPort("localhost", port)

	ip, err := fetchIP(context.Background(), endpoint, ipFamilyV6)
	require.NoError(t, err)
	require.Equal(t, "2001:db8::20", ip)

	ip, err = fetchIP(context.Background(), endpoint, ipFamilyV4)
	require.NoError(t, err)
	require.Equal(t, "198.51.100.12", ip)
}
