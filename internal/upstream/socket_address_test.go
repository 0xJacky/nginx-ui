package upstream

import (
	"slices"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSocketAddress(t *testing.T) {
	tests := []struct {
		server string
		want   string
	}{
		{"web.internal", "web.internal:80"},
		{"web.internal:8080", "web.internal:8080"},
		{"10.0.0.1", "10.0.0.1:80"},
		{"10.0.0.1:8080", "10.0.0.1:8080"},
		{"[::1]", "[::1]:80"},
		{"[::1]:8080", "[::1]:8080"},
		{"[2001:db8::1]", "[2001:db8::1]:80"},
		{"::1", "[::1]:80"},
		{"unix:/run/app.sock", "unix:/run/app.sock"},
		{"nohost.invalid", "nohost.invalid:80"},
		// Parameters after the address do not change the key.
		{"web.internal weight=2 max_fails=3 backup", "web.internal:80"},
		{"  10.0.0.1  down", "10.0.0.1:80"},
		{"nohost.invalid resolve", "nohost.invalid:80"},
		// Service discovery entries use the scanner's dynamic key.
		{"backend.service.consul service=api resolve", "service.consul:dynamic"},
		// Servers the scanner never registers have no key at all.
		{"$backend", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.server, func(t *testing.T) {
			assert.Equal(t, tt.want, SocketAddress(tt.server))
		})
	}
}

// recordingProber answers every probe as online and remembers which sockets
// the availability test asked for, so a test can see what the checker probes
// without dialing anything.
type recordingProber struct {
	mu     sync.Mutex
	probed []string
}

func (p *recordingProber) Probe(sockets []string) map[string]*Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make(map[string]*Status, len(sockets))
	for _, socket := range sockets {
		p.probed = append(p.probed, socket)
		result[socket] = &Status{Online: true, Latency: 1}
	}
	return result
}

// Every key SocketAddress hands out must be one the scanner registers and the
// availability test reports on; otherwise a list that looks health up by it
// shows "No Data" for a server that is being probed. Servers written without
// a port are the case that used to go wrong.
func TestSocketAddressMatchesProbedTargets(t *testing.T) {
	servers := []string{
		"web.internal",
		"10.0.0.1",
		"10.0.0.2:8080",
		"[::1]",
		"[::1]:8080",
		"unix:/run/app.sock",
		"nohost.invalid weight=2",
	}

	content := "upstream pool {\n"
	for _, server := range servers {
		content += "    server " + server + ";\n"
	}
	content += "}\n"

	service := GetUpstreamService()
	service.ClearTargets()
	t.Cleanup(service.ClearTargets)

	originalEnabled := settings.UpstreamCheckSettings.Enabled
	settings.UpstreamCheckSettings.Enabled = true
	t.Cleanup(func() { settings.UpstreamCheckSettings.Enabled = originalEnabled })

	prober := &recordingProber{}
	SetProber(prober)
	t.Cleanup(func() { SetProber(nil) })

	// Nothing to read from the database: treat every socket as enabled.
	service.disabledSocketsCacheMutex.Lock()
	service.cachedDisabledSockets = map[string]bool{}
	service.disabledSocketsCacheValid = true
	service.disabledSocketsCacheMutex.Unlock()
	t.Cleanup(service.InvalidateDisabledSocketsCache)

	require.NoError(t, ScanConfig("/etc/nginx/conf.d/upstream-pool.conf", []byte(content)))

	registered := make([]string, 0)
	for _, info := range service.GetTargetInfos() {
		registered = append(registered, formatSocketAddress(info.Host, info.Port))
	}

	service.PerformAvailabilityTest()
	results := service.GetAvailabilityMap()

	for _, server := range servers {
		socket := SocketAddress(server)
		require.NotEmpty(t, socket, server)
		assert.Contains(t, registered, socket, "scanner does not register %q under %q", server, socket)
		assert.True(t, slices.Contains(prober.probed, socket), "checker does not probe %q under %q", server, socket)
		assert.Contains(t, results, socket, "no availability result for %q under %q", server, socket)
	}
}
