package dns

import (
	"net/http"
	"time"
)

// OverrideIPEndpointsForTest swaps public IP detection endpoints for black-box
// service tests. It is compiled only during test builds.
//
// The replacement endpoints are httptest servers, which listen on 127.0.0.1
// only, while their bodies may carry an IPv6 address to stand in for the IPv6
// probe. The override therefore also stops pinning probes to their address
// family; otherwise every IPv6 probe would be refused before reaching them.
// Family pinning itself is covered by ddns_probe_test.go.
func OverrideIPEndpointsForTest(ipv4 []string, ipv6 []string) func() {
	originalIPv4Endpoints := ipv4Endpoints
	originalIPv6Endpoints := ipv6Endpoints
	ipv4Endpoints = ipv4
	ipv6Endpoints = ipv6
	restoreClients := useUnpinnedIPProbeClientsForTest()
	return func() {
		ipv4Endpoints = originalIPv4Endpoints
		ipv6Endpoints = originalIPv6Endpoints
		restoreClients()
	}
}

// useUnpinnedIPProbeClientsForTest makes every family probe dial over "tcp" so
// tests can serve any family's answer from a 127.0.0.1 httptest server.
// Returns a restore func.
func useUnpinnedIPProbeClientsForTest() func() {
	original := ipProbeClients
	unpinned := original[ipFamilyAny]
	ipProbeClients = map[ipFamily]*http.Client{
		ipFamilyAny: unpinned,
		ipFamilyV4:  unpinned,
		ipFamilyV6:  unpinned,
	}
	return func() { ipProbeClients = original }
}

// SetDDNSFamilyFailureGraceForTest overrides the grace period so tests can
// trigger the eviction window with very small durations. Returns a restore func.
func SetDDNSFamilyFailureGraceForTest(d time.Duration) func() {
	original := ddnsFamilyFailureGrace
	ddnsFamilyFailureGrace = d
	return func() { ddnsFamilyFailureGrace = original }
}
