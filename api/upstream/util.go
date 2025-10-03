package upstream

// formatSocketAddress formats a host:port combination into a proper socket address
// For IPv6 addresses, it adds brackets around the host if they're not already present
func formatSocketAddress(host, port string) string {
	// Reuse the logic from service package
	if len(host) > 0 && host[0] != '[' && containsColon(host) {
		return "[" + host + "]:" + port
	}
	return host + ":" + port
}

// containsColon checks if string contains a colon
func containsColon(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return true
		}
	}
	return false
}
