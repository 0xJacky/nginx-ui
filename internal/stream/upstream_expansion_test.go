package stream

import (
	"os"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
)

func TestBuildConfig_UpstreamExpansion(t *testing.T) {
	// Setup upstream service with test data
	service := upstream.GetUpstreamService()
	service.ClearTargets() // Clear any existing data

	// Add test upstream definitions
	tcpBackendServers := []upstream.ProxyTarget{
		{Host: "192.168.1.100", Port: "3306", Type: "upstream"},
		{Host: "192.168.1.101", Port: "3306", Type: "upstream"},
		{Host: "::1", Port: "3306", Type: "upstream"},
	}
	service.UpdateUpstreamDefinition("tcp_backend", tcpBackendServers, "test.conf")

	udpBackendServers := []upstream.ProxyTarget{
		{Host: "dns1.example.com", Port: "53", Type: "upstream"},
		{Host: "dns2.example.com", Port: "53", Type: "upstream"},
	}
	service.UpdateUpstreamDefinition("udp_backend", udpBackendServers, "test.conf")

	// Create a mock indexed stream with proxy targets that reference upstreams
	IndexedStreams["test_stream"] = &StreamIndex{
		Path:    "test_stream",
		Content: "test content",
		ProxyTargets: []upstream.ProxyTarget{
			{Host: "tcp_backend", Port: "80", Type: "proxy_pass"},          // This should be expanded
			{Host: "udp_backend", Port: "80", Type: "proxy_pass"},          // This should be expanded
			{Host: "direct.example.com", Port: "8080", Type: "proxy_pass"}, // This should remain as-is
		},
	}

	// Create mock file info
	fileInfo := &mockFileInfo{
		name:    "test_stream",
		size:    1024,
		modTime: time.Now(),
		isDir:   false,
	}

	// Call buildConfig
	result := buildConfig("test_stream", fileInfo, config.StatusEnabled, 0, nil)

	// Verify the results
	expectedTargetCount := 6 // 3 from tcp_backend + 2 from udp_backend + 1 direct
	if len(result.ProxyTargets) != expectedTargetCount {
		t.Errorf("Expected %d proxy targets, got %d", expectedTargetCount, len(result.ProxyTargets))
		for i, target := range result.ProxyTargets {
			t.Logf("Target %d: Host=%s, Port=%s, Type=%s", i, target.Host, target.Port, target.Type)
		}
	}

	// Check for specific targets
	expectedHosts := map[string]bool{
		"192.168.1.100":      false,
		"192.168.1.101":      false,
		"::1":                false,
		"dns1.example.com":   false,
		"dns2.example.com":   false,
		"direct.example.com": false,
	}

	for _, target := range result.ProxyTargets {
		if _, exists := expectedHosts[target.Host]; exists {
			expectedHosts[target.Host] = true
		}
	}

	// Verify all expected hosts were found
	for host, found := range expectedHosts {
		if !found {
			t.Errorf("Expected to find host %s in proxy targets", host)
		}
	}

	// Verify that upstream names are not present in the final targets
	for _, target := range result.ProxyTargets {
		if target.Host == "tcp_backend" || target.Host == "udp_backend" {
			t.Errorf("Upstream name %s should have been expanded, not included directly", target.Host)
		}
	}

	// Clean up
	delete(IndexedStreams, "test_stream")
}

func TestBuildConfig_NoUpstreamExpansion(t *testing.T) {
	// Test case where proxy targets don't reference any upstreams
	IndexedStreams["test_stream_no_upstream"] = &StreamIndex{
		Path:    "test_stream_no_upstream",
		Content: "test content",
		ProxyTargets: []upstream.ProxyTarget{
			{Host: "direct1.example.com", Port: "8080", Type: "proxy_pass"},
			{Host: "direct2.example.com", Port: "9000", Type: "proxy_pass"},
			{Host: "::1", Port: "3000", Type: "proxy_pass"},
		},
	}

	fileInfo := &mockFileInfo{
		name:    "test_stream_no_upstream",
		size:    1024,
		modTime: time.Now(),
		isDir:   false,
	}

	result := buildConfig("test_stream_no_upstream", fileInfo, config.StatusEnabled, 0, nil)

	// Should have exactly 3 targets, unchanged
	if len(result.ProxyTargets) != 3 {
		t.Errorf("Expected 3 proxy targets, got %d", len(result.ProxyTargets))
	}

	expectedTargets := []config.ProxyTarget{
		{Host: "direct1.example.com", Port: "8080", Type: "proxy_pass"},
		{Host: "direct2.example.com", Port: "9000", Type: "proxy_pass"},
		{Host: "::1", Port: "3000", Type: "proxy_pass"},
	}

	for i, expected := range expectedTargets {
		if i >= len(result.ProxyTargets) {
			t.Errorf("Missing target %d", i)
			continue
		}
		actual := result.ProxyTargets[i]
		if actual.Host != expected.Host || actual.Port != expected.Port || actual.Type != expected.Type {
			t.Errorf("Target %d mismatch: expected %+v, got %+v", i, expected, actual)
		}
	}

	// Clean up
	delete(IndexedStreams, "test_stream_no_upstream")
}

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }
