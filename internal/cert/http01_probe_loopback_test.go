package cert

import (
	"net"
	"net/http"
	"runtime"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubStatusBlock is the server block of issue #2073: a catch-all server on a
// specific loopback address that only serves stub_status, so it answers the
// challenge path with 404.
func stubStatusBlock(addr string, ipv6 bool) nginx.ServerBlock {
	return nginx.ServerBlock{
		File:        "/etc/nginx/conf.d/status.conf",
		ServerNames: []string{"_"},
		Listens:     []nginx.ServerListen{{Addr: addr, Port: "80", IPv6: ipv6}},
		Locations:   []nginx.ServerLocation{{Modifier: "=", Path: "/basic_status"}},
	}
}

// wildcardSiteBlock serves domain on the wildcard socket of one family and
// routes the challenge to proxyPass (no challenge location when empty).
func wildcardSiteBlock(domain string, ipv6 bool, proxyPass string) nginx.ServerBlock {
	block := nginx.ServerBlock{
		File:        "/etc/nginx/sites-enabled/" + domain,
		ServerNames: []string{domain},
		Listens:     []nginx.ServerListen{{Port: "80", IPv6: ipv6}},
	}
	if proxyPass != "" {
		block.Locations = []nginx.ServerLocation{http01ChallengeLocation(proxyPass)}
	}
	return block
}

// useLocalAddresses makes the probe see alternatives as the local addresses
// of this host, for both families, and counts the lookups.
func useLocalAddresses(alternatives map[bool][]string) *int {
	calls := 0
	http01ProbeLocalAddresses = func(ipv6 bool) []string {
		calls++
		return alternatives[ipv6]
	}
	return &calls
}

// The configuration of issue #2073: the domain is served by the wildcard
// socket, and a stub_status server owns 127.0.0.1:80. A request to
// 127.0.0.1:80 lands on the stub server ("*:80" and "127.0.0.1:80" both lead
// to it here), while the CA reaches the wildcard socket through the public
// address, played by 127.0.0.2.
func TestProbeHTTP01RoutesLoopbackListenerDoesNotShadowWildcardSocket(t *testing.T) {
	_, port := setupHTTP01Probe(t, nil, nil)
	stub := serveHTTP01Socket(t, http.NotFoundHandler(), false)
	wildcard := serveHTTP01Socket(t, challengeProxy(port), false)
	useHTTP01Blocks([]nginx.ServerBlock{
		wildcardSiteBlock("d.example.com", false, "http://127.0.0.1:"+port),
		stubStatusBlock("127.0.0.1", false),
	}, map[string]string{
		"*:80":         stub,
		"127.0.0.1:80": stub,
		"127.0.0.2:80": wildcard,
	})
	useLocalAddresses(map[bool][]string{false: {"127.0.0.2"}})

	results := probe(t, "d.example.com")

	assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status, results[0].Error)
	assert.Equal(t, wildcard, results[0].Target)
	require.Len(t, results[0].Attempts, 1, "the stub_status socket does not serve d.example.com")
	assert.NoError(t, HTTP01RouteCheckError(results, ""))
}

func TestProbeHTTP01RoutesShadowedWildcardSocketWithoutAlternativeIsWarning(t *testing.T) {
	_, port := setupHTTP01Probe(t, nil, nil)
	stub := serveHTTP01Socket(t, http.NotFoundHandler(), false)
	useHTTP01Blocks([]nginx.ServerBlock{
		wildcardSiteBlock("d.example.com", false, "http://127.0.0.1:"+port),
		stubStatusBlock("127.0.0.1", false),
	}, map[string]string{"*:80": stub, "127.0.0.1:80": stub})
	// macOS: only 127.0.0.1 is local and the host has no other address.
	useLocalAddresses(nil)

	results := probe(t, "d.example.com")

	assert.Equal(t, HTTP01ProbeStatusWarning, results[0].Status)
	assert.Equal(t, 1, results[0].Rounds, "a warning is not polled again")
	assert.Equal(t, "the wildcard socket *:80 cannot be reached locally: Nginx answers 127.0.0.1:80 with the server block listening on that address, and no other local address of this host reaches *:80, so the route cannot be verified locally", results[0].Error)
	assert.NoError(t, HTTP01RouteCheckError(results, ""))
}

func TestProbeHTTP01RoutesShadowedWildcardSocketSkipsOwnedAlternatives(t *testing.T) {
	_, port := setupHTTP01Probe(t, nil, nil)
	stub := serveHTTP01Socket(t, http.NotFoundHandler(), false)
	wildcard := serveHTTP01Socket(t, challengeProxy(port), false)
	useHTTP01Blocks([]nginx.ServerBlock{
		wildcardSiteBlock("d.example.com", false, "http://127.0.0.1:"+port),
		stubStatusBlock("127.0.0.1", false),
		stubStatusBlock("127.0.0.2", false),
	}, map[string]string{
		"*:80":          stub,
		"127.0.0.1:80":  stub,
		"127.0.0.2:80":  stub,
		"192.0.2.15:80": wildcard,
	})
	useLocalAddresses(map[bool][]string{false: {"127.0.0.2", "192.0.2.15"}})

	results := probe(t, "d.example.com")

	assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status, results[0].Error)
	assert.Equal(t, wildcard, results[0].Target)
}

func TestProbeHTTP01RoutesShadowedWildcardSocketStillReportsMisroute(t *testing.T) {
	// The wildcard site has no challenge location: dialed through the
	// alternative address, it still produces HTTP evidence of a wrong route.
	_, _ = setupHTTP01Probe(t, nil, nil)
	stub := serveHTTP01Socket(t, http.NotFoundHandler(), false)
	wildcard := serveHTTP01Socket(t, http.NotFoundHandler(), false)
	useHTTP01Blocks([]nginx.ServerBlock{
		wildcardSiteBlock("d.example.com", false, ""),
		stubStatusBlock("127.0.0.1", false),
	}, map[string]string{
		"*:80":         stub,
		"127.0.0.1:80": stub,
		"127.0.0.2:80": wildcard,
	})
	useLocalAddresses(map[bool][]string{false: {"127.0.0.2"}})

	results := probe(t, "d.example.com")

	assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
	assert.Equal(t, wildcard, results[0].Target)
	assert.Equal(t, http.StatusNotFound, results[0].StatusCode)
	err := HTTP01RouteCheckError(results, "")
	requireCosyCode(t, err, 50058)
	assert.Contains(t, err.Error(), "possible cause: the server block for d.example.com on *:80 in /etc/nginx/sites-enabled/d.example.com has no /.well-known/acme-challenge location")
	assert.NotContains(t, err.Error(), "status.conf")
}

func TestProbeHTTP01RoutesUnshadowedWildcardSocketUsesLoopback(t *testing.T) {
	_, port := setupHTTP01Probe(t, nil, nil)
	wildcard := serveHTTP01Socket(t, challengeProxy(port), false)
	useHTTP01Blocks([]nginx.ServerBlock{
		wildcardSiteBlock("d.example.com", false, "http://127.0.0.1:"+port),
		// A specific socket on another port or address does not shadow the
		// wildcard socket's 127.0.0.1:80.
		stubStatusBlock("192.0.2.15", false),
		{ServerNames: []string{"_"}, Listens: []nginx.ServerListen{{Addr: "127.0.0.1", Port: "8080"}}},
	}, map[string]string{"*:80": wildcard})
	calls := useLocalAddresses(map[bool][]string{false: {"127.0.0.2"}})

	results := probe(t, "d.example.com")

	assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status, results[0].Error)
	assert.Equal(t, wildcard, results[0].Target)
	assert.Zero(t, *calls, "alternative addresses are only looked up for a shadowed socket")
}

func TestProbeHTTP01RoutesExplicitLoopbackSiteIsDialedDirectly(t *testing.T) {
	t.Run("only the loopback socket serves the domain", func(t *testing.T) {
		_, port := setupHTTP01Probe(t, nil, nil)
		loopback := serveHTTP01Socket(t, challengeProxy(port), false)
		block := wildcardSiteBlock("d.example.com", false, "http://127.0.0.1:"+port)
		block.Listens = []nginx.ServerListen{{Addr: "127.0.0.1", Port: "80"}}
		useHTTP01Blocks([]nginx.ServerBlock{block}, map[string]string{"127.0.0.1:80": loopback})
		calls := useLocalAddresses(map[bool][]string{false: {"127.0.0.2"}})

		results := probe(t, "d.example.com")

		assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status, results[0].Error)
		assert.Equal(t, loopback, results[0].Target)
		assert.Zero(t, *calls)
	})

	t.Run("loopback and wildcard sockets both serve the domain", func(t *testing.T) {
		_, port := setupHTTP01Probe(t, nil, nil)
		loopback := serveHTTP01Socket(t, http.NotFoundHandler(), false)
		wildcard := serveHTTP01Socket(t, challengeProxy(port), false)
		block := wildcardSiteBlock("d.example.com", false, "http://127.0.0.1:"+port)
		block.Listens = append(block.Listens, nginx.ServerListen{Addr: "127.0.0.1", Port: "80"})
		useHTTP01Blocks([]nginx.ServerBlock{block}, map[string]string{
			"*:80":         loopback,
			"127.0.0.1:80": loopback,
			"127.0.0.2:80": wildcard,
		})
		useLocalAddresses(map[bool][]string{false: {"127.0.0.2"}})

		results := probe(t, "d.example.com")

		// Each socket is probed through an address only it owns; the 404 of
		// the loopback socket is real evidence.
		require.Len(t, results[0].Attempts, 2)
		assert.Equal(t, wildcard, results[0].Attempts[0].Target)
		assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Attempts[0].Status)
		assert.Equal(t, loopback, results[0].Attempts[1].Target)
		assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
	})
}

func TestProbeHTTP01RoutesIPv6LoopbackListener(t *testing.T) {
	t.Run("shadowed wildcard socket is dialed through another address", func(t *testing.T) {
		_, port := setupHTTP01Probe(t, nil, nil)
		stub := serveHTTP01Socket(t, http.NotFoundHandler(), false)
		wildcard := serveHTTP01Socket(t, challengeProxy(port), false)
		useHTTP01Blocks([]nginx.ServerBlock{
			wildcardSiteBlock("d.example.com", true, "http://127.0.0.1:"+port),
			stubStatusBlock("[::1]", true),
		}, map[string]string{
			"[::]:80":          stub,
			"[::1]:80":         stub,
			"[2001:db8::5]:80": wildcard,
			"127.0.0.2:80":     stub,
		})
		useLocalAddresses(map[bool][]string{false: {"127.0.0.2"}, true: {"2001:db8::5"}})

		results := probe(t, "d.example.com")

		assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status, results[0].Error)
		assert.Equal(t, wildcard, results[0].Target)
	})

	t.Run("without an alternative it is a warning", func(t *testing.T) {
		_, port := setupHTTP01Probe(t, nil, nil)
		stub := serveHTTP01Socket(t, http.NotFoundHandler(), false)
		useHTTP01Blocks([]nginx.ServerBlock{
			wildcardSiteBlock("d.example.com", true, "http://127.0.0.1:"+port),
			stubStatusBlock("::1", true),
		}, map[string]string{"[::]:80": stub, "[::1]:80": stub})
		useLocalAddresses(map[bool][]string{false: {"127.0.0.2"}})

		results := probe(t, "d.example.com")

		assert.Equal(t, HTTP01ProbeStatusWarning, results[0].Status)
		assert.Contains(t, results[0].Error, "the wildcard socket [::]:80 cannot be reached locally: Nginx answers [::1]:80")
		assert.NoError(t, HTTP01RouteCheckError(results, ""))
	})

	t.Run("unshadowed IPv6 wildcard socket uses [::1]", func(t *testing.T) {
		_, port := setupHTTP01Probe(t, nil, nil)
		wildcard := serveHTTP01Socket(t, challengeProxy(port), false)
		useHTTP01Blocks([]nginx.ServerBlock{
			wildcardSiteBlock("d.example.com", true, "http://127.0.0.1:"+port),
			// The IPv4 loopback socket does not shadow the IPv6 wildcard one.
			stubStatusBlock("127.0.0.1", false),
		}, map[string]string{"[::]:80": wildcard})
		calls := useLocalAddresses(nil)

		results := probe(t, "d.example.com")

		assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status, results[0].Error)
		assert.Zero(t, *calls)
	})
}

func TestProbeHTTP01RoutesShadowedWildcardSocketOnRedirectHop(t *testing.T) {
	_, port := setupHTTP01Probe(t, nil, nil)
	http80 := serveHTTP01Socket(t, redirectTo("https://d.example.com%s"), false)
	stub := serveHTTP01Socket(t, http.NotFoundHandler(), true)
	wildcard := serveHTTP01Socket(t, challengeProxy(port), true)
	useHTTP01Blocks([]nginx.ServerBlock{
		{
			File:        "/etc/nginx/sites-enabled/d",
			ServerNames: []string{"d.example.com"},
			Listens:     []nginx.ServerListen{{Port: "80"}, {Port: "443", SSL: true}},
			Return:      "301 https://$host$request_uri",
		},
		{
			File:        "/etc/nginx/conf.d/status.conf",
			ServerNames: []string{"_"},
			Listens:     []nginx.ServerListen{{Addr: "127.0.0.1", Port: "443", SSL: true}},
		},
	}, map[string]string{
		"*:80":          http80,
		"*:443":         stub,
		"127.0.0.1:443": stub,
		"127.0.0.2:443": wildcard,
	})
	useLocalAddresses(map[bool][]string{false: {"127.0.0.2"}})

	results := probe(t, "d.example.com")

	assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status, results[0].Error)
	assert.Equal(t, http80, results[0].Target, "only the port-443 hop is shadowed")
	require.Len(t, results[0].Redirects, 1)
	assert.Contains(t, results[0].Redirects[0], "https://d.example.com"+http01ProbePathPrefix)
}

func TestHTTP01SpecificListenAddresses(t *testing.T) {
	owned := http01SpecificListenAddresses([]nginx.ServerBlock{{Listens: []nginx.ServerListen{
		{Port: "80"},
		{Addr: "*", Port: "80"},
		{Addr: "0.0.0.0", Port: "80"},
		{Port: "80", IPv6: true},
		{Addr: "127.0.0.1", Port: "80"},
		{Addr: "[::1]", Port: "80", IPv6: true},
		{Addr: "192.0.2.10", Port: "443", SSL: true},
		{Addr: "unix:/run/nginx.sock"},
		{Addr: "2001:DB8:0::1"},
	}}}, "80")

	assert.Equal(t, map[string]bool{"127.0.0.1": true, "::1": true, "2001:db8::1": true}, owned)

	localhost := http01SpecificListenAddresses([]nginx.ServerBlock{{Listens: []nginx.ServerListen{{Addr: "localhost", Port: "80"}}}}, "80")
	assert.Equal(t, map[string]bool{"127.0.0.1": true, "::1": true}, localhost)
}

func TestHTTP01LocalAlternativeAddresses(t *testing.T) {
	for _, ipv6 := range []bool{false, true} {
		for _, address := range http01LocalAlternativeAddresses(ipv6) {
			ip := net.ParseIP(address)
			require.NotNil(t, ip, address)
			assert.Equal(t, ipv6, ip.To4() == nil, address)
			assert.NotEqual(t, "127.0.0.1", address)
			assert.NotEqual(t, "::1", address)
		}
	}
	if runtime.GOOS == "linux" {
		// Linux routes the whole 127.0.0.0/8 to the loopback interface.
		assert.Contains(t, http01LocalAlternativeAddresses(false), "127.0.0.2")
	}
}
