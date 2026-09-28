package upstream

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	internalUpstream "github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serverSockets(t *testing.T, servers []any) map[string]string {
	t.Helper()
	sockets := make(map[string]string, len(servers))
	for _, item := range servers {
		server := item.(map[string]any)
		sockets[server["address"].(string)] = server["socket"].(string)
	}
	return sockets
}

// Both tables of the upstream list look health up by the socket the response
// carries, so managed groups and upstream blocks from other files must report
// the same key for the same server, and it must be the key the checker uses.
func TestListReportsSameSocketsForManagedAndExternalServers(t *testing.T) {
	router, confDir := managedAPI(t)

	status, resp := doJSON(t, router, http.MethodPost, "/upstreams",
		`{"name":"web_pool","servers":[`+
			`{"address":"web.internal"},{"address":"10.0.0.1"},{"address":"10.0.0.2:8080"},`+
			`{"address":"[::1]"},{"address":"[::1]:8080"},{"address":"unix:/run/app.sock"},`+
			`{"address":"nohost.invalid","params":"resolve"}]}`)
	require.Equal(t, http.StatusOK, status, resp)

	otherPath := filepath.Join(confDir, "conf.d", "legacy.conf")
	content := "upstream legacy_pool {\n    zone legacy_pool 64k;\n    server web.internal;\n    server 10.0.0.1;\n" +
		"    server 10.0.0.2:8080;\n    server [::1];\n    server [::1]:8080;\n    server unix:/run/app.sock;\n" +
		"    server nohost.invalid resolve;\n}\n"
	require.NoError(t, os.WriteFile(otherPath, []byte(content), 0o644))
	require.NoError(t, internalUpstream.ScanConfig(otherPath, []byte(content)))

	want := map[string]string{
		"web.internal":       "web.internal:80",
		"10.0.0.1":           "10.0.0.1:80",
		"10.0.0.2:8080":      "10.0.0.2:8080",
		"[::1]":              "[::1]:80",
		"[::1]:8080":         "[::1]:8080",
		"unix:/run/app.sock": "unix:/run/app.sock",
		"nohost.invalid":     "nohost.invalid:80",
	}

	status, resp = doJSON(t, router, http.MethodGet, "/upstreams", "")
	require.Equal(t, http.StatusOK, status, resp)

	data := resp["data"].([]any)
	require.Len(t, data, 1)
	managedGroup := data[0].(map[string]any)
	assert.Equal(t, "web_pool", managedGroup["name"])
	assert.Equal(t, want, serverSockets(t, managedGroup["servers"].([]any)))

	external := findExternal(t, resp, "legacy_pool")
	assert.Equal(t, want, serverSockets(t, external["servers"].([]any)))

	// Every reported socket is a target the checker probes.
	probed := make(map[string]bool)
	for _, info := range internalUpstream.GetUpstreamService().GetTargetInfos() {
		probed[formatSocketAddress(info.Host, info.Port)] = true
	}
	for address, socket := range want {
		assert.True(t, probed[socket], "%s is not probed as %s", address, socket)
	}
}
