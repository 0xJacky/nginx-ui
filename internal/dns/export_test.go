package dns

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
