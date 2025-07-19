package upstream

import (
	"testing"
)

// TestParseProxyTargetsWithConsulResolver tests parsing of nginx config with consul DNS resolver
func TestParseProxyTargetsWithConsulResolver(t *testing.T) {
	config := `upstream redacted-net {
    zone upstream_web 128k;
    resolver 127.0.0.1:8600 valid=5s;
    resolver_timeout 2s;
    server service.consul service=redacted-net resolve;
}
server {
    listen 80;
    listen [::]:80;
    server_name redacted.net;
    location / {
        proxy_pass http://redacted-net;
    }
}`

	targets := ParseProxyTargetsFromRawContent(config)

	// Print actual results for debugging
	t.Logf("Found %d targets:", len(targets))
	for i, target := range targets {
		t.Logf("Target %d: Host=%s, Port=%s, Type=%s, Resolver=%s, IsConsul=%v",
			i+1, target.Host, target.Port, target.Type, target.Resolver, target.IsConsul)
	}

	// Expected behavior:
	// - Should parse "service.consul" as host with dynamic port
	// - Should identify this as an upstream target with consul service discovery
	// - Should capture resolver information
	// - proxy_pass http://redacted-net should be ignored since it references upstream
	expectedTargets := []ProxyTarget{
		{
			Host:       "service.consul",
			Port:       "dynamic",
			Type:       "upstream",
			Resolver:   "127.0.0.1:8600",
			IsConsul:   true,
			ServiceURL: "service.consul service=redacted-net resolve",
		},
	}

	if len(targets) != len(expectedTargets) {
		t.Errorf("Expected %d targets, got %d", len(expectedTargets), len(targets))
		return
	}

	// Create a map for easier comparison
	targetMap := make(map[string]ProxyTarget)
	for _, target := range targets {
		key := target.Host + ":" + target.Port + ":" + target.Type + ":" + target.Resolver
		if target.IsConsul {
			key += ":consul:" + target.ServiceURL
		}
		targetMap[key] = target
	}

	for _, expected := range expectedTargets {
		key := expected.Host + ":" + expected.Port + ":" + expected.Type + ":" + expected.Resolver
		if expected.IsConsul {
			key += ":consul:" + expected.ServiceURL
		}
		if _, found := targetMap[key]; !found {
			t.Errorf("Expected target not found: %+v", expected)
		}
	}
}

// TestConsulResolverExtractServiceName tests service name extraction
func TestConsulResolverExtractServiceName(t *testing.T) {
	resolver := NewConsulResolver("127.0.0.1:8600")

	tests := []struct {
		serviceURL   string
		expectedName string
	}{
		{
			serviceURL:   "service.consul service=redacted-net resolve",
			expectedName: "redacted-net",
		},
		{
			serviceURL:   "service.consul service=web-service resolve",
			expectedName: "web-service",
		},
		{
			serviceURL:   "service.consul service=api-backend resolve",
			expectedName: "api-backend",
		},
		{
			serviceURL:   "my-service.service.consul",
			expectedName: "my-service",
		},
		{
			serviceURL:   "invalid-format",
			expectedName: "",
		},
	}

	for _, test := range tests {
		result := resolver.extractServiceName(test.serviceURL)
		if result != test.expectedName {
			t.Errorf("extractServiceName(%q) = %q, expected %q", test.serviceURL, result, test.expectedName)
		}
	}
}

// TestConsulResolverResolveService tests the actual resolution functionality
func TestConsulResolverResolveService(t *testing.T) {
	// Test with a mock resolver that should fail (127.0.0.1:8600 is consul default but likely not running)
	t.Run("ResolutionWithMockConsul", func(t *testing.T) {
		resolver := NewConsulResolver("127.0.0.1:8600")

		addresses, err := resolver.ResolveService("service.consul service=test-service resolve")

		// We expect this to fail since there's no real consul server
		if err == nil {
			t.Logf("Unexpected success: resolved addresses %v (maybe there's a real consul server?)", addresses)
		} else {
			t.Logf("Expected failure: %v", err)
		}
	})

	// Test with invalid service URL
	t.Run("InvalidServiceURL", func(t *testing.T) {
		resolver := NewConsulResolver("127.0.0.1:8600")

		addresses, err := resolver.ResolveService("invalid-service-url")

		if err == nil {
			t.Errorf("Expected error for invalid service URL, got addresses: %v", addresses)
		}

		if len(addresses) != 0 {
			t.Errorf("Expected no addresses for invalid service URL, got %v", addresses)
		}
	})

	// Test with invalid resolver address
	t.Run("InvalidResolverAddress", func(t *testing.T) {
		resolver := NewConsulResolver("192.168.254.254:8600") // Unreachable IP

		addresses, err := resolver.ResolveService("service.consul service=test-service resolve")

		// Should fail due to unreachable resolver
		if err == nil {
			t.Errorf("Expected error for unreachable resolver, got addresses: %v", addresses)
		}
	})

	// Test service name extraction edge cases
	t.Run("ServiceNameExtractionEdgeCases", func(t *testing.T) {
		resolver := NewConsulResolver("127.0.0.1:8600")

		testCases := []struct {
			serviceURL   string
			expectedName string
		}{
			{"", ""},
			{"service.consul", ""},
			{"service.consul resolve", ""},
			{"service.consul service= resolve", ""},
			{"service.consul service=  resolve", ""}, // Empty service name
			{"my-service.service.consul", "my-service"},
			{"complex-service-name.service.consul", "complex-service-name"},
		}

		for _, tc := range testCases {
			result := resolver.extractServiceName(tc.serviceURL)
			if result != tc.expectedName {
				t.Errorf("extractServiceName(%q) = %q, expected %q", tc.serviceURL, result, tc.expectedName)
			}
		}
	})
}

// TestTestConsulTargets tests the new dedicated consul testing function
func TestTestConsulTargets(t *testing.T) {
	// Test 1: Valid consul targets with resolver
	t.Run("ValidConsulTargets", func(t *testing.T) {
		consulTargets := []ProxyTarget{
			{
				Host:       "service.consul",
				Port:       "dynamic",
				Type:       "upstream",
				Resolver:   "127.0.0.1:8600",
				IsConsul:   true,
				ServiceURL: "service.consul service=test-service resolve",
			},
		}

		results := TestConsulTargets(consulTargets)

		// Should have exactly 1 result
		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}

		// Check the result exists with correct key
		key := "service.consul:dynamic"
		if status, found := results[key]; found {
			// The status doesn't matter much (likely offline without real consul)
			// What matters is that the function processes the target correctly
			t.Logf("Consul target %s processed: Online=%v, Latency=%.2f", key, status.Online, status.Latency)
		} else {
			t.Errorf("Expected result for key %s not found", key)
		}
	})

	// Test 2: Consul target without resolver should be marked offline
	t.Run("ConsulTargetWithoutResolver", func(t *testing.T) {
		consulTargets := []ProxyTarget{
			{
				Host: "service.consul",
				Port: "dynamic",
				Type: "upstream",
				// No resolver - should be marked offline immediately
				IsConsul:   true,
				ServiceURL: "service.consul service=no-resolver resolve",
			},
		}

		results := TestConsulTargets(consulTargets)

		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}

		key := "service.consul:dynamic"
		if status, found := results[key]; found {
			if status.Online {
				t.Errorf("Expected consul target without resolver to be offline, but it's online")
			}
			if status.Latency != 0 {
				t.Errorf("Expected latency to be 0 for offline target, got %.2f", status.Latency)
			}
		} else {
			t.Errorf("Expected result for key %s not found", key)
		}
	})

	// Test 3: Multiple consul targets with different resolvers
	t.Run("MultipleConsulTargetsWithDifferentResolvers", func(t *testing.T) {
		consulTargets := []ProxyTarget{
			{
				Host:       "web-service.consul",
				Port:       "dynamic",
				Type:       "upstream",
				Resolver:   "127.0.0.1:8600",
				IsConsul:   true,
				ServiceURL: "service.consul service=web-service resolve",
			},
			{
				Host:       "api-service.consul",
				Port:       "dynamic",
				Type:       "upstream",
				Resolver:   "127.0.0.1:8500", // Different resolver
				IsConsul:   true,
				ServiceURL: "service.consul service=api-service resolve",
			},
		}

		results := TestConsulTargets(consulTargets)

		// Should have 2 results
		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}

		// Check both results exist
		expectedKeys := []string{
			"web-service.consul:dynamic",
			"api-service.consul:dynamic",
		}

		for _, key := range expectedKeys {
			if _, found := results[key]; !found {
				t.Errorf("Expected result for key %s not found", key)
			}
		}
	})

	// Test 4: Empty consul targets should return empty results
	t.Run("EmptyConsulTargets", func(t *testing.T) {
		consulTargets := []ProxyTarget{}
		results := TestConsulTargets(consulTargets)

		if len(results) != 0 {
			t.Errorf("Expected 0 results for empty targets, got %d", len(results))
		}
	})
}

// TestSimplifiedArchitecture tests the new simplified architecture
func TestSimplifiedArchitecture(t *testing.T) {
	service := GetUpstreamService()
	service.ClearTargets()

	// Mix of traditional and consul targets
	mixedTargets := []ProxyTarget{
		// Traditional targets
		{Host: "127.0.0.1", Port: "80", Type: "upstream"},
		{Host: "192.168.1.100", Port: "8080", Type: "upstream"},
		// Consul targets
		{
			Host:       "service.consul",
			Port:       "dynamic",
			Type:       "upstream",
			Resolver:   "127.0.0.1:8600",
			IsConsul:   true,
			ServiceURL: "service.consul service=my-service resolve",
		},
	}

	service.updateTargetsFromConfig("test-config.conf", mixedTargets)

	// Verify targets are correctly stored
	service.targetsMutex.RLock()
	traditionalCount := 0
	consulCount := 0
	for _, targetInfo := range service.targets {
		if targetInfo.ProxyTarget.IsConsul {
			consulCount++
		} else {
			traditionalCount++
		}
	}
	service.targetsMutex.RUnlock()

	if traditionalCount != 2 {
		t.Errorf("Expected 2 traditional targets, got %d", traditionalCount)
	}
	if consulCount != 1 {
		t.Errorf("Expected 1 consul target, got %d", consulCount)
	}

	t.Logf("Architecture correctly separated %d traditional and %d consul targets", traditionalCount, consulCount)

	// Clean up
	service.ClearTargets()
}

// TestEnhancedAvailabilityTest tests the enhanced availability testing with mixed targets
func TestEnhancedAvailabilityTest(t *testing.T) {
	targets := []ProxyTarget{
		// Regular target
		{
			Host: "127.0.0.1",
			Port: "22", // SSH port might be available
			Type: "upstream",
		},
		// Consul target (will fail since no real consul)
		{
			Host:       "service.consul",
			Port:       "dynamic",
			Type:       "upstream",
			Resolver:   "127.0.0.1:8600",
			IsConsul:   true,
			ServiceURL: "service.consul service=test-service resolve",
		},
		// Invalid regular target
		{
			Host: "192.168.254.254",
			Port: "9999",
			Type: "upstream",
		},
	}

	results := EnhancedAvailabilityTest(targets)

	t.Logf("Found %d test results:", len(results))
	for key, status := range results {
		t.Logf("Target %s: Online=%v, Latency=%.2f", key, status.Online, status.Latency)
	}

	// Verify we have results for all targets
	expectedKeys := []string{
		"127.0.0.1:22",
		"service.consul:dynamic",
		"192.168.254.254:9999",
	}

	for _, key := range expectedKeys {
		if _, found := results[key]; !found {
			t.Errorf("Expected result for key %s not found", key)
		}
	}

	// Test that consul target is processed (result doesn't matter, we just verify the flow works)
	if status, found := results["service.consul:dynamic"]; found {
		t.Logf("Consul target processed: Online=%v, Latency=%.2f", status.Online, status.Latency)
	}

	// Test that unreachable target is properly handled
	if status, found := results["192.168.254.254:9999"]; found {
		// This should be offline due to unreachable IP, but we verify the function handles it
		t.Logf("Unreachable target processed: Online=%v, Latency=%.2f", status.Online, status.Latency)
		// Most likely offline, but we don't assume - we just verify it was processed
	}
}

// TestTraditionalAvailabilityTestUsage tests that traditional AvailabilityTest is used when no consul targets
func TestTraditionalAvailabilityTestUsage(t *testing.T) {
	// Test with only traditional targets
	targets := []ProxyTarget{
		{
			Host: "127.0.0.1",
			Port: "80",
			Type: "upstream",
		},
		{
			Host: "192.168.254.254",
			Port: "9999",
			Type: "upstream",
		},
	}

	// Test enhanced version with traditional targets only
	enhancedResults := EnhancedAvailabilityTest(targets)

	// Test traditional version directly
	traditionalKeys := []string{"127.0.0.1:80", "192.168.254.254:9999"}
	traditionalResults := AvailabilityTest(traditionalKeys)

	t.Logf("Enhanced results: %d items", len(enhancedResults))
	t.Logf("Traditional results: %d items", len(traditionalResults))

	// Both should have same number of results
	if len(enhancedResults) != len(traditionalResults) {
		t.Errorf("Expected same number of results, enhanced=%d, traditional=%d",
			len(enhancedResults), len(traditionalResults))
		return
	}

	// Results should be consistent
	for key, traditionalStatus := range traditionalResults {
		if enhancedStatus, found := enhancedResults[key]; found {
			if traditionalStatus.Online != enhancedStatus.Online {
				t.Errorf("Inconsistent online status for %s: traditional=%v, enhanced=%v",
					key, traditionalStatus.Online, enhancedStatus.Online)
			}
		} else {
			t.Errorf("Key %s missing in enhanced results", key)
		}
	}

	t.Logf("Enhanced test correctly delegated to traditional test for non-consul targets")
}

// TestUpstreamServiceSimplifiedFlow tests the new simplified flow in UpstreamService
func TestUpstreamServiceSimplifiedFlow(t *testing.T) {
	service := GetUpstreamService()
	service.ClearTargets()

	// Add mixed targets
	mixedTargets := []ProxyTarget{
		{Host: "127.0.0.1", Port: "80", Type: "upstream"},
		{
			Host:       "service.consul",
			Port:       "dynamic",
			Type:       "upstream",
			Resolver:   "127.0.0.1:8600",
			IsConsul:   true,
			ServiceURL: "service.consul service=test resolve",
		},
	}

	service.updateTargetsFromConfig("test-config.conf", mixedTargets)

	// This would trigger the simplified flow
	service.PerformAvailabilityTest()

	results := service.GetAvailabilityMap()
	t.Logf("Simplified flow generated %d results:", len(results))
	for key, status := range results {
		t.Logf("Result %s: Online=%v, Latency=%.2f", key, status.Online, status.Latency)
	}

	// Should have results for both targets
	expectedKeys := []string{"127.0.0.1:80", "service.consul:dynamic"}
	for _, key := range expectedKeys {
		if _, found := results[key]; !found {
			t.Errorf("Expected result for key %s not found", key)
		}
	}

	t.Logf("Simplified architecture correctly processed mixed targets")

	// Clean up
	service.ClearTargets()
}
