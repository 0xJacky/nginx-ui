package managed

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// socketsOf returns the socket of every server of a detail, in order.
func socketsOf(detail *Detail) []string {
	sockets := make([]string, 0, len(detail.Servers))
	for _, server := range detail.Servers {
		sockets = append(sockets, server.Socket)
	}
	return sockets
}

// A managed server is shown with the health the checker reports for it, and
// the checker keys that by the socket it probes. The detail therefore carries
// that socket for every server, with port 80 filled in where the address has
// none, instead of leaving the frontend to look health up by the address.
func TestDetailCarriesResolvedSockets(t *testing.T) {
	setupStoreTest(t)

	u := pool("web_pool",
		"web.internal",
		"10.0.0.1",
		"10.0.0.2:8080",
		"[::1]",
		"[::1]:8080",
		"unix:/run/app.sock",
	)
	u.Servers = append(u.Servers, Server{Address: "nohost.invalid", Params: "resolve", Backup: true})
	want := []string{
		"web.internal:80",
		"10.0.0.1:80",
		"10.0.0.2:8080",
		"[::1]:80",
		"[::1]:8080",
		"unix:/run/app.sock",
		"nohost.invalid:80",
	}

	saved, err := Save(u, true, "admin")
	require.NoError(t, err)
	assert.Equal(t, want, socketsOf(saved))

	got, err := Get("web_pool")
	require.NoError(t, err)
	assert.Equal(t, want, socketsOf(got))

	list, err := List()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, want, socketsOf(list[0]))

	// Each socket is a target the availability checker probes after the save.
	probed := make(map[string]bool)
	for _, info := range upstream.GetUpstreamService().GetTargetInfos() {
		probed[upstream.SocketAddress(net.JoinHostPort(info.Host, info.Port))] = true
	}
	for i, socket := range want {
		assert.True(t, probed[socket], "server %q is not probed as %q", saved.Servers[i].Address, socket)
	}
}

// The socket is derived, never taken from the request: a client echoing back
// a stale or made-up value changes neither the file nor the reported key.
func TestSaveIgnoresSocketFromRequest(t *testing.T) {
	confDir := setupStoreTest(t)

	u := pool("echo_pool", "10.0.0.1")
	u.Servers[0].Socket = "10.9.9.9:1234"

	detail, err := Save(u, true, "admin")
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1:80"}, socketsOf(detail))

	content, err := os.ReadFile(filepath.Join(confDir, "conf.d", FileName("echo_pool")))
	require.NoError(t, err)
	assert.NotContains(t, string(content), "10.9.9.9")

	// The structured form round-trips without the derived field.
	parsed, err := Parse("echo_pool", string(content))
	require.NoError(t, err)
	assert.Empty(t, parsed.Servers[0].Socket)
}
