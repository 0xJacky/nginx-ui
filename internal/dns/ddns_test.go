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

func TestSanitizeDDNSIPVersion(t *testing.T) {
	t.Run("accepts supported values", func(t *testing.T) {
		for _, tc := range []struct {
			input string
			want  string
		}{
			{input: "ipv4", want: DDNSIPVersionIPv4},
			{input: " IPv6 ", want: DDNSIPVersionIPv6},
			{input: "ipv4_ipv6", want: DDNSIPVersionIPv4IPv6},
			{input: "IPv6_IPv4", want: DDNSIPVersionIPv6IPv4},
			{input: "both_required", want: DDNSIPVersionBothRequired},
		} {
			version, err := sanitizeDDNSIPVersion(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.want, version)
		}
	})

	t.Run("rejects empty and unsupported values", func(t *testing.T) {
		for _, input := range []string{"", "both", "ipv10"} {
			_, err := sanitizeDDNSIPVersion(input)
			require.ErrorIs(t, err, ErrInvalidDDNSIPVersion)
		}
	})
}

func TestNormalizeDDNSIPVersion(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "", want: DDNSIPVersionIPv4IPv6},
		{input: "both", want: DDNSIPVersionIPv4IPv6},
		{input: "invalid", want: DDNSIPVersionIPv4IPv6},
		{input: " IPv6_IPv4 ", want: DDNSIPVersionIPv6IPv4},
	} {
		require.Equal(t, tc.want, NormalizeDDNSIPVersion(tc.input))
	}
}

func TestResolvePublicIPsRespectsIPVersion(t *testing.T) {
	t.Run("ipv4 mode does not require ipv6", func(t *testing.T) {
		ipv4Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("198.51.100.12"))
		}))
		defer ipv4Server.Close()

		restore := OverrideIPEndpointsForTest([]string{ipv4Server.URL}, []string{"http://127.0.0.1:1"})
		defer restore()

		snapshot, err := resolvePublicIPs(context.Background(), DDNSIPVersionIPv4)
		require.NoError(t, err)
		require.Equal(t, "198.51.100.12", snapshot.IPv4)
		require.Empty(t, snapshot.IPv6)
	})

	t.Run("ipv6 mode does not require ipv4", func(t *testing.T) {
		ipv6Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("2001:db8::20"))
		}))
		defer ipv6Server.Close()

		restore := OverrideIPEndpointsForTest([]string{"http://127.0.0.1:1"}, []string{ipv6Server.URL})
		defer restore()

		snapshot, err := resolvePublicIPs(context.Background(), DDNSIPVersionIPv6)
		require.NoError(t, err)
		require.Empty(t, snapshot.IPv4)
		require.Equal(t, "2001:db8::20", snapshot.IPv6)
	})

	t.Run("ipv4 ipv6 mode tolerates ipv6 failure", func(t *testing.T) {
		ipv4Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("198.51.100.12"))
		}))
		defer ipv4Server.Close()

		restore := OverrideIPEndpointsForTest([]string{ipv4Server.URL}, []string{"http://127.0.0.1:1"})
		defer restore()

		snapshot, err := resolvePublicIPs(context.Background(), DDNSIPVersionIPv4IPv6)
		require.NoError(t, err)
		require.Equal(t, "198.51.100.12", snapshot.IPv4)
		require.Empty(t, snapshot.IPv6)
	})

	t.Run("ipv6 ipv4 mode tolerates ipv4 failure", func(t *testing.T) {
		ipv6Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("2001:db8::20"))
		}))
		defer ipv6Server.Close()

		restore := OverrideIPEndpointsForTest([]string{"http://127.0.0.1:1"}, []string{ipv6Server.URL})
		defer restore()

		snapshot, err := resolvePublicIPs(context.Background(), DDNSIPVersionIPv6IPv4)
		require.NoError(t, err)
		require.Empty(t, snapshot.IPv4)
		require.Equal(t, "2001:db8::20", snapshot.IPv6)
	})

	t.Run("both required fails when either family fails", func(t *testing.T) {
		ipv4Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("198.51.100.12"))
		}))
		defer ipv4Server.Close()

		restore := OverrideIPEndpointsForTest([]string{ipv4Server.URL}, []string{"http://127.0.0.1:1"})
		defer restore()

		snapshot, err := resolvePublicIPs(context.Background(), DDNSIPVersionBothRequired)
		require.Error(t, err)
		require.Equal(t, "198.51.100.12", snapshot.IPv4)
		require.Empty(t, snapshot.IPv6)
	})
}

func TestDDNSIPVersionMatchesRecordType(t *testing.T) {
	require.True(t, ddnsIPVersionMatchesRecordType(DDNSIPVersionIPv4, "A"))
	require.False(t, ddnsIPVersionMatchesRecordType(DDNSIPVersionIPv4, "AAAA"))
	require.True(t, ddnsIPVersionMatchesRecordType(DDNSIPVersionIPv6, "AAAA"))
	require.False(t, ddnsIPVersionMatchesRecordType(DDNSIPVersionIPv6, "A"))
	require.True(t, ddnsIPVersionMatchesRecordType(DDNSIPVersionIPv4IPv6, "A"))
	require.True(t, ddnsIPVersionMatchesRecordType(DDNSIPVersionIPv4IPv6, "AAAA"))
	require.True(t, ddnsIPVersionMatchesRecordType(DDNSIPVersionIPv6IPv4, "A"))
	require.True(t, ddnsIPVersionMatchesRecordType(DDNSIPVersionIPv6IPv4, "AAAA"))
	require.True(t, ddnsIPVersionMatchesRecordType(DDNSIPVersionBothRequired, "A"))
	require.True(t, ddnsIPVersionMatchesRecordType(DDNSIPVersionBothRequired, "AAAA"))
	require.False(t, ddnsIPVersionMatchesRecordType("invalid", "TXT"))
}
