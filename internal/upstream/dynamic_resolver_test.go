package upstream

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"testing"
)

// MockDNSServer simulates DNS responses for testing
type MockDNSServer struct {
	srvRecords map[string][]*net.SRV
	aRecords   map[string][]net.IPAddr
}

// NewMockDNSServer creates a mock DNS server for testing
func NewMockDNSServer() *MockDNSServer {
	return &MockDNSServer{
		srvRecords: make(map[string][]*net.SRV),
		aRecords:   make(map[string][]net.IPAddr),
	}
}

// AddSRVRecord adds a SRV record to the mock DNS server
func (m *MockDNSServer) AddSRVRecord(domain string, priority, weight uint16, port uint16, target string) {
	m.srvRecords[domain] = append(m.srvRecords[domain], &net.SRV{
		Priority: priority,
		Weight:   weight,
		Port:     port,
		Target:   target,
	})
}

// AddARecord adds an A record to the mock DNS server
func (m *MockDNSServer) AddARecord(domain string, ip string) {
	m.aRecords[domain] = append(m.aRecords[domain], net.IPAddr{
		IP: net.ParseIP(ip),
	})
}

// MockResolver is a custom resolver that uses our mock DNS server
type MockResolver struct {
	mockServer *MockDNSServer
}

// LookupSRV simulates SRV record lookup with proper priority sorting
func (mr *MockResolver) LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error) {
	domain := name
	if service != "" || proto != "" {
		domain = fmt.Sprintf("_%s._%s.%s", service, proto, name)
	}

	if records, exists := mr.mockServer.srvRecords[domain]; exists {
		// Sort SRV records by priority (lowest first), then by weight (highest first)
		// This follows RFC 2782 and nginx behavior
		sortedRecords := make([]*net.SRV, len(records))
		copy(sortedRecords, records)

		sort.Slice(sortedRecords, func(i, j int) bool {
			if sortedRecords[i].Priority != sortedRecords[j].Priority {
				return sortedRecords[i].Priority < sortedRecords[j].Priority
			}
			// For same priority, higher weight comes first (but this is simplified for testing)
			return sortedRecords[i].Weight > sortedRecords[j].Weight
		})

		return "", sortedRecords, nil
	}
	return "", nil, fmt.Errorf("no SRV records for %s", domain)
}

// LookupIPAddr simulates A record lookup
func (mr *MockResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	if records, exists := mr.mockServer.aRecords[host]; exists {
		return records, nil
	}
	return nil, fmt.Errorf("no A records for %s", host)
}

// TestParseServiceURL tests the parseServiceURL function with nginx compliance
func TestParseServiceURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedErr bool
		expected    *ServiceInfo
	}{
		{
			name:  "Valid nginx service URL - simple service name",
			input: "backend.example.com service=http resolve",
			expected: &ServiceInfo{
				Hostname:    "backend.example.com",
				ServiceName: "http",
			},
		},
		{
			name:  "Valid nginx service URL - service name with underscores",
			input: "backend.example.com service=_http._tcp resolve",
			expected: &ServiceInfo{
				Hostname:    "backend.example.com",
				ServiceName: "_http._tcp",
			},
		},
		{
			name:  "Valid nginx service URL - service name with dots",
			input: "example.com service=server1.backend resolve",
			expected: &ServiceInfo{
				Hostname:    "example.com",
				ServiceName: "server1.backend",
			},
		},
		{
			name:  "Consul service example",
			input: "service.consul service=web-service resolve",
			expected: &ServiceInfo{
				Hostname:    "service.consul",
				ServiceName: "web-service",
			},
		},
		{
			name:        "Empty input",
			input:       "",
			expectedErr: true,
		},
		{
			name:        "Missing resolve parameter",
			input:       "backend.example.com service=http",
			expectedErr: true,
		},
		{
			name:        "Missing service parameter",
			input:       "backend.example.com resolve",
			expectedErr: true,
		},
		{
			name:        "Empty service name",
			input:       "backend.example.com service= resolve",
			expectedErr: true,
		},
		{
			name:        "Only hostname",
			input:       "backend.example.com",
			expectedErr: true,
		},
	}

	resolver := NewDynamicResolver("127.0.0.1:8600")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.parseServiceURL(tt.input)

			if tt.expectedErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.Hostname != tt.expected.Hostname {
				t.Errorf("Expected hostname %s, got %s", tt.expected.Hostname, result.Hostname)
			}

			if result.ServiceName != tt.expected.ServiceName {
				t.Errorf("Expected service name %s, got %s", tt.expected.ServiceName, result.ServiceName)
			}
		})
	}
}

// TestConstructSRVDomain tests SRV domain construction according to nginx.org rules
func TestConstructSRVDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    *ServiceInfo
		expected string
		rule     string
	}{
		{
			name: "Rule 1: Service name without dots - http",
			input: &ServiceInfo{
				Hostname:    "backend.example.com",
				ServiceName: "http",
			},
			expected: "_http._tcp.backend.example.com",
			rule:     "nginx rule 1: no dots, add TCP protocol",
		},
		{
			name: "Rule 1: Service name without dots - https",
			input: &ServiceInfo{
				Hostname:    "api.example.com",
				ServiceName: "https",
			},
			expected: "_https._tcp.api.example.com",
			rule:     "nginx rule 1: no dots, add TCP protocol",
		},
		{
			name: "Rule 1: Service name without dots - mysql",
			input: &ServiceInfo{
				Hostname:    "db.example.com",
				ServiceName: "mysql",
			},
			expected: "_mysql._tcp.db.example.com",
			rule:     "nginx rule 1: no dots, add TCP protocol",
		},
		{
			name: "Rule 2: Service name with dots - _http._tcp",
			input: &ServiceInfo{
				Hostname:    "backend.example.com",
				ServiceName: "_http._tcp",
			},
			expected: "_http._tcp.backend.example.com",
			rule:     "nginx rule 2: contains dots, join directly",
		},
		{
			name: "Rule 2: Service name with dots - server1.backend",
			input: &ServiceInfo{
				Hostname:    "example.com",
				ServiceName: "server1.backend",
			},
			expected: "server1.backend.example.com",
			rule:     "nginx rule 2: contains dots, join directly",
		},
		{
			name: "Rule 2: Complex service name with underscores and dots",
			input: &ServiceInfo{
				Hostname:    "dc1.consul",
				ServiceName: "_api._tcp.production",
			},
			expected: "_api._tcp.production.dc1.consul",
			rule:     "nginx rule 2: contains dots, join directly",
		},
		{
			name: "Consul example - simple service",
			input: &ServiceInfo{
				Hostname:    "service.consul",
				ServiceName: "web",
			},
			expected: "_web._tcp.service.consul",
			rule:     "nginx rule 1: no dots, add TCP protocol",
		},
	}

	resolver := NewDynamicResolver("127.0.0.1:8600")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.constructSRVDomain(tt.input)
			if result != tt.expected {
				t.Errorf("Expected SRV domain %s, got %s (rule: %s)", tt.expected, result, tt.rule)
			}
		})
	}
}

// TestNginxOfficialExamples tests the exact examples from nginx.org documentation
func TestNginxOfficialExamples(t *testing.T) {
	tests := []struct {
		name          string
		nginxConfig   string
		expectedQuery string
		description   string
	}{
		{
			name:          "Official Example 1",
			nginxConfig:   "backend.example.com service=http resolve",
			expectedQuery: "_http._tcp.backend.example.com",
			description:   "To look up _http._tcp.backend.example.com SRV record",
		},
		{
			name:          "Official Example 2",
			nginxConfig:   "backend.example.com service=_http._tcp resolve",
			expectedQuery: "_http._tcp.backend.example.com",
			description:   "Service name already contains dots, join directly",
		},
		{
			name:          "Official Example 3",
			nginxConfig:   "example.com service=server1.backend resolve",
			expectedQuery: "server1.backend.example.com",
			description:   "Service name contains dots, join directly",
		},
	}

	resolver := NewDynamicResolver("127.0.0.1:8600")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serviceInfo, err := resolver.parseServiceURL(tt.nginxConfig)
			if err != nil {
				t.Fatalf("Failed to parse nginx config: %v", err)
			}

			result := resolver.constructSRVDomain(serviceInfo)
			if result != tt.expectedQuery {
				t.Errorf("nginx.org example failed: expected %s, got %s (%s)",
					tt.expectedQuery, result, tt.description)
			}
		})
	}
}

// TestSRVRecordResolutionWithMockDNS tests actual SRV record resolution using mock DNS
func TestSRVRecordResolutionWithMockDNS(t *testing.T) {
	// Create mock DNS server
	mockDNS := NewMockDNSServer()

	// Add SRV records for _http._tcp.backend.example.com
	mockDNS.AddSRVRecord("_http._tcp.backend.example.com", 10, 60, 8080, "web1.backend.example.com")
	mockDNS.AddSRVRecord("_http._tcp.backend.example.com", 10, 40, 8080, "web2.backend.example.com")
	mockDNS.AddSRVRecord("_http._tcp.backend.example.com", 20, 100, 8080, "web3.backend.example.com")

	// Add A records for the targets
	mockDNS.AddARecord("web1.backend.example.com", "192.168.1.10")
	mockDNS.AddARecord("web2.backend.example.com", "192.168.1.11")
	mockDNS.AddARecord("web3.backend.example.com", "192.168.1.12")

	t.Run("SRV record resolution", func(t *testing.T) {
		mockResolver := &MockResolver{mockServer: mockDNS}

		// Test SRV lookup
		_, srvRecords, err := mockResolver.LookupSRV(context.Background(), "", "", "_http._tcp.backend.example.com")
		if err != nil {
			t.Fatalf("SRV lookup failed: %v", err)
		}

		if len(srvRecords) != 3 {
			t.Errorf("Expected 3 SRV records, got %d", len(srvRecords))
		}

		// Verify priority ordering (lowest priority first) and weight ordering (highest weight first within same priority)
		expectedPriorities := []uint16{10, 10, 20}
		expectedWeights := []uint16{60, 40, 100} // For priorities [10, 10, 20], weights should be [60, 40, 100]
		expectedTargets := []string{"web1.backend.example.com", "web2.backend.example.com", "web3.backend.example.com"}

		for i, srv := range srvRecords {
			if srv.Priority != expectedPriorities[i] {
				t.Errorf("Expected priority %d at index %d, got %d", expectedPriorities[i], i, srv.Priority)
			}
			if srv.Weight != expectedWeights[i] {
				t.Errorf("Expected weight %d at index %d, got %d", expectedWeights[i], i, srv.Weight)
			}
			if srv.Target != expectedTargets[i] {
				t.Errorf("Expected target %s at index %d, got %s", expectedTargets[i], i, srv.Target)
			}
		}

		// Test A record resolution for each target
		for _, srv := range srvRecords {
			ips, err := mockResolver.LookupIPAddr(context.Background(), srv.Target)
			if err != nil {
				t.Errorf("A record lookup failed for %s: %v", srv.Target, err)
				continue
			}

			if len(ips) != 1 {
				t.Errorf("Expected 1 IP for %s, got %d", srv.Target, len(ips))
			}
		}
	})
}

// TestSRVPriorityHandling tests nginx SRV priority handling as per nginx.org documentation
func TestSRVPriorityHandling(t *testing.T) {
	// Create mock DNS server
	mockDNS := NewMockDNSServer()

	// Add SRV records with different priorities to test primary/backup server logic
	// Priority 5 (highest priority / primary servers)
	mockDNS.AddSRVRecord("_http._tcp.app.example.com", 5, 100, 8080, "primary1.app.example.com")
	mockDNS.AddSRVRecord("_http._tcp.app.example.com", 5, 50, 8080, "primary2.app.example.com")
	// Priority 10 (backup servers)
	mockDNS.AddSRVRecord("_http._tcp.app.example.com", 10, 80, 8080, "backup1.app.example.com")
	// Priority 15 (lower priority backup servers)
	mockDNS.AddSRVRecord("_http._tcp.app.example.com", 15, 200, 8080, "backup2.app.example.com")

	// Add A records
	mockDNS.AddARecord("primary1.app.example.com", "10.0.1.1")
	mockDNS.AddARecord("primary2.app.example.com", "10.0.1.2")
	mockDNS.AddARecord("backup1.app.example.com", "10.0.2.1")
	mockDNS.AddARecord("backup2.app.example.com", "10.0.3.1")

	t.Run("SRV priority handling", func(t *testing.T) {
		mockResolver := &MockResolver{mockServer: mockDNS}

		// Test SRV lookup
		_, srvRecords, err := mockResolver.LookupSRV(context.Background(), "", "", "_http._tcp.app.example.com")
		if err != nil {
			t.Fatalf("SRV lookup failed: %v", err)
		}

		if len(srvRecords) != 4 {
			t.Errorf("Expected 4 SRV records, got %d", len(srvRecords))
		}

		// According to nginx.org: "Highest-priority SRV records (records with the same lowest-number priority value)
		// are resolved as primary servers, the rest of SRV records are resolved as backup servers"
		expectedOrder := []struct {
			priority   uint16
			weight     uint16
			target     string
			serverType string
		}{
			{5, 100, "primary1.app.example.com", "primary"}, // Highest priority (lowest number)
			{5, 50, "primary2.app.example.com", "primary"},  // Same priority, lower weight
			{10, 80, "backup1.app.example.com", "backup"},   // Lower priority (backup)
			{15, 200, "backup2.app.example.com", "backup"},  // Lowest priority (backup)
		}

		for i, srv := range srvRecords {
			expected := expectedOrder[i]
			if srv.Priority != expected.priority {
				t.Errorf("Record %d: expected priority %d, got %d", i, expected.priority, srv.Priority)
			}
			if srv.Weight != expected.weight {
				t.Errorf("Record %d: expected weight %d, got %d", i, expected.weight, srv.Weight)
			}
			if srv.Target != expected.target {
				t.Errorf("Record %d: expected target %s, got %s", i, expected.target, srv.Target)
			}

			// Log the server type for documentation
			t.Logf("Record %d: Priority %d, Weight %d, Target %s (%s server)",
				i, srv.Priority, srv.Weight, srv.Target, expected.serverType)
		}

		// Verify primary servers come first (lowest priority numbers)
		primaryCount := 0
		for _, srv := range srvRecords {
			if srv.Priority == 5 { // Primary servers have priority 5
				primaryCount++
			} else {
				break // Once we hit a non-primary, all following should be backups
			}
		}

		if primaryCount != 2 {
			t.Errorf("Expected 2 primary servers at the beginning, got %d", primaryCount)
		}
	})
}

// TestARecordFallback tests A record fallback when SRV lookup fails
func TestARecordFallback(t *testing.T) {
	mockDNS := NewMockDNSServer()

	// Only add A record, no SRV record
	mockDNS.AddARecord("_http._tcp.backend.example.com", "192.168.1.100")

	t.Run("A record fallback", func(t *testing.T) {
		mockResolver := &MockResolver{mockServer: mockDNS}

		// SRV lookup should fail
		_, srvRecords, err := mockResolver.LookupSRV(context.Background(), "", "", "_http._tcp.backend.example.com")
		if err == nil {
			t.Error("Expected SRV lookup to fail")
		}
		if len(srvRecords) != 0 {
			t.Errorf("Expected 0 SRV records, got %d", len(srvRecords))
		}

		// A record lookup should succeed
		ips, err := mockResolver.LookupIPAddr(context.Background(), "_http._tcp.backend.example.com")
		if err != nil {
			t.Fatalf("A record lookup failed: %v", err)
		}

		if len(ips) != 1 {
			t.Errorf("Expected 1 IP, got %d", len(ips))
		}

		expectedIP := "192.168.1.100"
		if ips[0].IP.String() != expectedIP {
			t.Errorf("Expected IP %s, got %s", expectedIP, ips[0].IP.String())
		}
	})
}

// TestComplexNginxScenarios tests more complex real-world nginx scenarios
func TestComplexNginxScenarios(t *testing.T) {
	tests := []struct {
		name        string
		nginxLine   string
		expectedSRV string
		scenario    string
	}{
		{
			name:        "Load balancer with HTTP service",
			nginxLine:   "api.microservices.local service=http resolve",
			expectedSRV: "_http._tcp.api.microservices.local",
			scenario:    "Microservices API load balancing",
		},
		{
			name:        "Database connection",
			nginxLine:   "db.cluster.local service=mysql resolve",
			expectedSRV: "_mysql._tcp.db.cluster.local",
			scenario:    "Database cluster connection",
		},
		{
			name:        "WebSocket service",
			nginxLine:   "chat.app.local service=ws resolve",
			expectedSRV: "_ws._tcp.chat.app.local",
			scenario:    "WebSocket service discovery",
		},
		{
			name:        "Custom protocol with dots",
			nginxLine:   "service.consul service=_grpc._tcp resolve",
			expectedSRV: "_grpc._tcp.service.consul",
			scenario:    "gRPC service via Consul",
		},
		{
			name:        "Multi-level service hierarchy",
			nginxLine:   "consul.local service=api.v1.production resolve",
			expectedSRV: "api.v1.production.consul.local",
			scenario:    "Multi-level service naming",
		},
		{
			name:        "Kubernetes style service",
			nginxLine:   "cluster.local service=_http._tcp.nginx.default resolve",
			expectedSRV: "_http._tcp.nginx.default.cluster.local",
			scenario:    "Kubernetes service discovery",
		},
	}

	resolver := NewDynamicResolver("127.0.0.1:8600")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serviceInfo, err := resolver.parseServiceURL(tt.nginxLine)
			if err != nil {
				t.Fatalf("Failed to parse nginx line: %v", err)
			}

			result := resolver.constructSRVDomain(serviceInfo)
			if result != tt.expectedSRV {
				t.Errorf("Scenario '%s' failed: expected %s, got %s",
					tt.scenario, tt.expectedSRV, result)
			}
		})
	}
}

// TestBackwardCompatibility tests backward compatibility with old format
func TestBackwardCompatibility(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "New nginx format should work",
			input:    "backend.example.com service=http resolve",
			expected: "http",
		},
		{
			name:     "New nginx format with dots",
			input:    "example.com service=_http._tcp resolve",
			expected: "_http._tcp",
		},
		{
			name:     "Old consul format should still work as fallback",
			input:    "test-service.service.consul",
			expected: "test-service",
		},
		{
			name:     "Invalid format should return empty",
			input:    "invalid format without proper structure",
			expected: "",
		},
	}

	resolver := NewDynamicResolver("127.0.0.1:8600")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.extractServiceName(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestDynamicTargetsFunction tests the TestDynamicTargets function
func TestDynamicTargetsFunction(t *testing.T) {
	t.Run("Valid dynamic targets", func(t *testing.T) {
		targets := []ProxyTarget{
			{
				Host:       "service.consul",
				Port:       "dynamic",
				Type:       "upstream",
				Resolver:   "127.0.0.1:8600",
				IsConsul:   true,
				ServiceURL: "backend.example.com service=http resolve",
			},
		}

		results := TestDynamicTargets(targets)

		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}

		key := "service.consul:dynamic"
		if _, found := results[key]; !found {
			t.Errorf("Expected result for key %s not found", key)
		}
	})

	t.Run("Target without resolver should be offline", func(t *testing.T) {
		targets := []ProxyTarget{
			{
				Host:       "service.consul",
				Port:       "dynamic",
				Type:       "upstream",
				IsConsul:   true,
				ServiceURL: "backend.example.com service=http resolve",
				// No resolver specified
			},
		}

		results := TestDynamicTargets(targets)

		key := "service.consul:dynamic"
		if status, found := results[key]; found {
			if status.Online {
				t.Error("Expected target without resolver to be offline")
			}
			if status.Latency != 0 {
				t.Errorf("Expected latency 0 for offline target, got %.2f", status.Latency)
			}
		} else {
			t.Errorf("Expected result for key %s", key)
		}
	})
}

// TestIntegrationWithProxyParser tests integration with the proxy parser
func TestIntegrationWithProxyParser(t *testing.T) {
	config := `upstream web-backend {
    zone upstream_web 128k;
    resolver 127.0.0.1:8600 valid=5s;
    resolver_timeout 2s;
    server backend.example.com service=http resolve;
}
server {
    listen 80;
    server_name example.com;
    location / {
        proxy_pass http://web-backend;
    }
}`

	targets := ParseProxyTargetsFromRawContent(config)

	// Should find the dynamic DNS target
	found := false
	for _, target := range targets {
		if target.IsConsul && strings.Contains(target.ServiceURL, "service=http") {
			found = true

			// Verify the target is correctly parsed
			if target.Resolver != "127.0.0.1:8600" {
				t.Errorf("Expected resolver 127.0.0.1:8600, got %s", target.Resolver)
			}

			if target.ServiceURL != "backend.example.com service=http resolve" {
				t.Errorf("Expected service URL 'backend.example.com service=http resolve', got %s", target.ServiceURL)
			}
			break
		}
	}

	if !found {
		t.Error("Dynamic DNS target not found in parsed config")
	}
}
