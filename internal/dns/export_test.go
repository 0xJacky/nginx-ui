package dns

import "time"

// OverrideIPEndpointsForTest swaps public IP detection endpoints for black-box
// service tests. It is compiled only during test builds.
func OverrideIPEndpointsForTest(ipv4 []string, ipv6 []string) func() {
	originalIPv4Endpoints := ipv4Endpoints
	originalIPv6Endpoints := ipv6Endpoints
	ipv4Endpoints = ipv4
	ipv6Endpoints = ipv6
	return func() {
		ipv4Endpoints = originalIPv4Endpoints
		ipv6Endpoints = originalIPv6Endpoints
	}
}

// SetDDNSFamilyFailureGraceForTest overrides the grace period so tests can
// trigger the eviction window with very small durations. Returns a restore func.
func SetDDNSFamilyFailureGraceForTest(d time.Duration) func() {
	original := ddnsFamilyFailureGrace
	ddnsFamilyFailureGrace = d
	return func() { ddnsFamilyFailureGrace = original }
}
