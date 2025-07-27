package upstream

import (
	"testing"
)

func TestParseAddressOnly_IPv6Support(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ProxyTarget
	}{
		// IPv6 with brackets and port
		{
			name:  "IPv6 with brackets and port",
			input: "[::1]:8080",
			expected: ProxyTarget{
				Host: "::1",
				Port: "8080",
			},
		},
		{
			name:  "IPv6 full address with brackets and port",
			input: "[2001:db8::1]:9000",
			expected: ProxyTarget{
				Host: "2001:db8::1",
				Port: "9000",
			},
		},
		// IPv6 with brackets without port
		{
			name:  "IPv6 with brackets without port",
			input: "[::1]",
			expected: ProxyTarget{
				Host: "::1",
				Port: "80",
			},
		},
		{
			name:  "IPv6 full address with brackets without port",
			input: "[2001:db8::1]",
			expected: ProxyTarget{
				Host: "2001:db8::1",
				Port: "80",
			},
		},
		// IPv6 without brackets
		{
			name:  "IPv6 localhost without brackets",
			input: "::1",
			expected: ProxyTarget{
				Host: "::1",
				Port: "80",
			},
		},
		{
			name:  "IPv6 full address without brackets",
			input: "2001:db8::1",
			expected: ProxyTarget{
				Host: "2001:db8::1",
				Port: "80",
			},
		},
		{
			name:  "IPv6 link-local with interface",
			input: "fe80::1%eth0",
			expected: ProxyTarget{
				Host: "fe80::1%eth0",
				Port: "80",
			},
		},
		// IPv4 tests
		{
			name:  "IPv4 with port",
			input: "192.168.1.1:8080",
			expected: ProxyTarget{
				Host: "192.168.1.1",
				Port: "8080",
			},
		},
		{
			name:  "IPv4 without port",
			input: "192.168.1.1",
			expected: ProxyTarget{
				Host: "192.168.1.1",
				Port: "80",
			},
		},
		// Hostname tests
		{
			name:  "Hostname with port",
			input: "example.com:8080",
			expected: ProxyTarget{
				Host: "example.com",
				Port: "8080",
			},
		},
		{
			name:  "Hostname without port",
			input: "example.com",
			expected: ProxyTarget{
				Host: "example.com",
				Port: "80",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAddressOnly(tt.input)
			if result.Host != tt.expected.Host {
				t.Errorf("parseAddressOnly(%q).Host = %q, want %q", tt.input, result.Host, tt.expected.Host)
			}
			if result.Port != tt.expected.Port {
				t.Errorf("parseAddressOnly(%q).Port = %q, want %q", tt.input, result.Port, tt.expected.Port)
			}
		})
	}
}

func TestParseProxyTargetsFromRawContent_IPv6Support(t *testing.T) {
	config := `
upstream backend_ipv6 {
    server [::1]:8080;
    server [2001:db8::1]:9000;
    server 192.168.1.100:8080;
}

upstream backend_mixed {
    server [::1]:8080;
    server 192.168.1.100:8080;
    server example.com:9000;
}

server {
    listen 80;
    location / {
        proxy_pass http://[::1]:8080;
    }
    location /api {
        proxy_pass http://backend_ipv6;
    }
}
`

	targets := ParseProxyTargetsFromRawContent(config)

	// Expected targets (after deduplication):
	// - [::1]:8080 (appears in both upstreams and proxy_pass, but deduplicated)
	// - [2001:db8::1]:9000 (from backend_ipv6)
	// - 192.168.1.100:8080 (appears in both upstreams, but deduplicated)
	// - example.com:9000 (from backend_mixed)
	// - [::1]:8080 proxy_pass (different type, so not deduplicated)
	expectedCount := 5
	if len(targets) != expectedCount {
		t.Errorf("Expected %d targets, got %d", expectedCount, len(targets))
		for i, target := range targets {
			t.Logf("Target %d: Host=%s, Port=%s, Type=%s", i, target.Host, target.Port, target.Type)
		}
	}

	// Check for IPv6 targets
	hasIPv6Localhost := false
	hasIPv6Full := false
	hasIPv4 := false

	for _, target := range targets {
		if target.Host == "::1" && target.Port == "8080" {
			hasIPv6Localhost = true
		}
		if target.Host == "2001:db8::1" && target.Port == "9000" {
			hasIPv6Full = true
		}
		if target.Host == "192.168.1.100" && target.Port == "8080" {
			hasIPv4 = true
		}
	}

	if !hasIPv6Localhost {
		t.Error("Expected to find IPv6 localhost target [::1]:8080")
	}
	if !hasIPv6Full {
		t.Error("Expected to find IPv6 full address target [2001:db8::1]:9000")
	}
	if !hasIPv4 {
		t.Error("Expected to find IPv4 target 192.168.1.100:8080")
	}
}
