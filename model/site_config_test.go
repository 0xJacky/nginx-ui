package model

import "testing"

func TestSiteConfigSetFromURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantScheme string
		wantHost   string
		wantPort   int
	}{
		{name: "http default", url: "http://example.com/path", wantScheme: "http", wantHost: "example.com:80", wantPort: 80},
		{name: "https explicit", url: "https://example.com:8443/path", wantScheme: "https", wantHost: "example.com:8443", wantPort: 8443},
		{name: "IPv6 default", url: "https://[2001:db8::1]/health", wantScheme: "https", wantHost: "[2001:db8::1]:443", wantPort: 443},
		{name: "IPv6 explicit", url: "grpc://[2001:db8::2]:50051/service", wantScheme: "grpc", wantHost: "[2001:db8::2]:50051", wantPort: 50051},
		{name: "scheme omitted", url: "example.com:8080/status", wantScheme: "http", wantHost: "example.com:8080", wantPort: 8080},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &SiteConfig{}
			if err := config.SetFromURL(test.url); err != nil {
				t.Fatalf("SetFromURL() error = %v", err)
			}
			if config.Scheme != test.wantScheme || config.Host != test.wantHost || config.Port != test.wantPort {
				t.Fatalf("SetFromURL() = scheme %q, host %q, port %d; want %q, %q, %d", config.Scheme, config.Host, config.Port, test.wantScheme, test.wantHost, test.wantPort)
			}
			if config.DisplayURL != test.url {
				t.Fatalf("DisplayURL = %q, want original %q", config.DisplayURL, test.url)
			}
		})
	}
}

func TestSiteConfigSetFromURLRejectsInvalidInputs(t *testing.T) {
	for _, rawURL := range []string{"ftp://example.com", "http://", "https://example.com:70000"} {
		config := &SiteConfig{}
		if err := config.SetFromURL(rawURL); err == nil {
			t.Errorf("SetFromURL(%q) error = nil", rawURL)
		}
	}
}
