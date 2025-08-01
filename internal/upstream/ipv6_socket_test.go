package upstream

import (
	"sync"
	"testing"
)

func TestFormatSocketAddress_IPv6(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		expected string
	}{
		{
			name:     "IPv6 all addresses",
			host:     "::",
			port:     "9001",
			expected: "[::]:9001",
		},
		{
			name:     "IPv6 localhost",
			host:     "::1",
			port:     "8080",
			expected: "[::1]:8080",
		},
		{
			name:     "IPv6 full address",
			host:     "2001:db8::1",
			port:     "9000",
			expected: "[2001:db8::1]:9000",
		},
		{
			name:     "IPv6 with brackets already",
			host:     "[::1]",
			port:     "8080",
			expected: "[::1]:8080",
		},
		{
			name:     "IPv4 address",
			host:     "127.0.0.1",
			port:     "9001",
			expected: "127.0.0.1:9001",
		},
		{
			name:     "hostname",
			host:     "example.com",
			port:     "80",
			expected: "example.com:80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSocketAddress(tt.host, tt.port)
			if result != tt.expected {
				t.Errorf("formatSocketAddress(%q, %q) = %q, want %q", tt.host, tt.port, result, tt.expected)
			}
		})
	}
}

func TestAvailabilityTest_IPv6Socket(t *testing.T) {
	// Test that IPv6 socket addresses are properly formatted
	// This test verifies that the socket string passed to net.DialTimeout is correct

	// Test with properly formatted IPv6 addresses
	sockets := []string{
		"[::1]:8080",     // IPv6 localhost with port
		"127.0.0.1:8080", // IPv4 for comparison
	}

	// This should not panic or cause parsing errors
	results := AvailabilityTest(sockets)

	// Verify we get results for both sockets (even if they're offline)
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Check that the keys are preserved correctly
	for _, socket := range sockets {
		if _, exists := results[socket]; !exists {
			t.Errorf("Expected result for socket %q", socket)
		}
	}
}

func TestTCPLatency_IPv6Support(t *testing.T) {
	// Test that testTCPLatency can handle IPv6 addresses correctly
	// Note: This test verifies the function doesn't panic with IPv6 addresses
	// The actual connection will likely fail since we're testing non-existent services

	tests := []struct {
		name   string
		socket string
	}{
		{
			name:   "IPv6 localhost",
			socket: "[::1]:8080",
		},
		{
			name:   "IPv6 all addresses",
			socket: "[::]:9001",
		},
		{
			name:   "IPv4 for comparison",
			socket: "127.0.0.1:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wg sync.WaitGroup
			status := &Status{}

			wg.Add(1)

			// This should not panic even if the connection fails
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("testTCPLatency panicked with socket %q: %v", tt.socket, r)
				}
			}()

			testTCPLatency(&wg, tt.socket, status)
			wg.Wait()

			// We don't check if it's online since the service likely doesn't exist
			// We just verify the function completed without panicking
		})
	}
}
