package dns

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseIPStringRespectsAddressFamily(t *testing.T) {
	t.Run("rejects ipv4 for ipv6", func(t *testing.T) {
		_, err := parseIPString("203.0.113.10", ipFamilyV6)
		require.Error(t, err)
	})

	t.Run("accepts embedded ipv6 text", func(t *testing.T) {
		ip, err := parseIPString("Current IP: 2001:db8::10", ipFamilyV6)
		require.NoError(t, err)
		require.Equal(t, "2001:db8::10", ip)
	})

	t.Run("rejects ipv6 for ipv4", func(t *testing.T) {
		_, err := parseIPString("2001:db8::10", ipFamilyV4)
		require.Error(t, err)
	})
}

func TestFetchAnyIPSkipsMismatchedAddressFamily(t *testing.T) {
	ipv4Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("198.51.100.12"))
	}))
	defer ipv4Server.Close()

	ipv6Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("2001:db8::20"))
	}))
	defer ipv6Server.Close()

	ip, err := fetchAnyIP(context.Background(), []string{ipv4Server.URL, ipv6Server.URL}, ipFamilyV6)
	require.NoError(t, err)
	require.Equal(t, "2001:db8::20", ip)
}
