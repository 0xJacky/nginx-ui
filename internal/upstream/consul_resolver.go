package upstream

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// ConsulResolver handles DNS resolution through consul
type ConsulResolver struct {
	resolver string // e.g., "127.0.0.1:8600"
}

// NewConsulResolver creates a new consul resolver
func NewConsulResolver(resolver string) *ConsulResolver {
	return &ConsulResolver{
		resolver: resolver,
	}
}

// ResolveService resolves a consul service to actual IP addresses and ports
func (cr *ConsulResolver) ResolveService(serviceURL string) ([]string, error) {
	// Parse consul service URL (e.g., "service.consul service=redacted-net resolve")
	serviceName := cr.extractServiceName(serviceURL)
	if serviceName == "" {
		return nil, fmt.Errorf("could not extract service name from: %s", serviceURL)
	}

	// Create a custom resolver that uses the consul DNS server
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, cr.resolver)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Query consul for service SRV records
	_, srvRecords, err := resolver.LookupSRV(ctx, "", "", serviceName+".service.consul")
	if err != nil {
		// Fallback to A record lookup if SRV fails
		ips, err := resolver.LookupIPAddr(ctx, serviceName+".service.consul")
		if err != nil {
			return nil, fmt.Errorf("failed to resolve service %s: %v", serviceName, err)
		}

		// Return IP addresses with default port (80)
		var addresses []string
		for _, ip := range ips {
			addresses = append(addresses, fmt.Sprintf("%s:80", ip.IP.String()))
		}
		return addresses, nil
	}

	// Convert SRV records to address:port format
	var addresses []string
	for _, srv := range srvRecords {
		// Resolve the target hostname to IP
		ips, err := resolver.LookupIPAddr(ctx, srv.Target)
		if err != nil {
			continue // Skip this record if resolution fails
		}

		for _, ip := range ips {
			addresses = append(addresses, fmt.Sprintf("%s:%d", ip.IP.String(), srv.Port))
		}
	}

	if len(addresses) == 0 {
		return nil, fmt.Errorf("no addresses found for service %s", serviceName)
	}

	return addresses, nil
}

// extractServiceName extracts the service name from consul service URL
func (cr *ConsulResolver) extractServiceName(serviceURL string) string {
	serviceURL = strings.TrimSpace(serviceURL)

	// Handle empty input
	if serviceURL == "" {
		return ""
	}

	// Parse "service.consul service=redacted-net resolve" format
	if strings.Contains(serviceURL, "service=") {
		parts := strings.Fields(serviceURL)
		for _, part := range parts {
			if strings.HasPrefix(part, "service=") {
				serviceName := strings.TrimPrefix(part, "service=")
				// Handle edge cases like "service=" or "service=  "
				serviceName = strings.TrimSpace(serviceName)
				if serviceName == "" {
					return ""
				}
				return serviceName
			}
		}
	}

	// Fallback: try to extract from hostname format like "my-service.service.consul"
	if strings.Contains(serviceURL, ".service.consul") {
		parts := strings.Split(serviceURL, ".")
		if len(parts) > 0 {
			serviceName := strings.TrimSpace(parts[0])
			if serviceName == "" {
				return ""
			}
			return serviceName
		}
	}

	return ""
}

// TestConsulTargets performs availability test specifically for consul targets
func TestConsulTargets(consulTargets []ProxyTarget) map[string]*Status {
	result := make(map[string]*Status)

	// Group consul targets by resolver
	consulTargetsByResolver := make(map[string][]ProxyTarget)
	for _, target := range consulTargets {
		if target.Resolver != "" {
			consulTargetsByResolver[target.Resolver] = append(consulTargetsByResolver[target.Resolver], target)
		} else {
			// No resolver specified, mark as offline
			key := target.Host + ":" + target.Port
			result[key] = &Status{
				Online:  false,
				Latency: 0,
			}
		}
	}

	// Test each resolver group
	for resolver, targets := range consulTargetsByResolver {
		consulResolver := NewConsulResolver(resolver)

		for _, target := range targets {
			key := target.Host + ":" + target.Port

			// Try to resolve the consul service
			addresses, err := consulResolver.ResolveService(target.ServiceURL)
			if err != nil {
				// If resolution fails, mark as offline
				result[key] = &Status{
					Online:  false,
					Latency: 0,
				}
				continue
			}

			// Test the first resolved address as representative
			if len(addresses) > 0 {
				addressResults := AvailabilityTest(addresses[:1])

				if status, exists := addressResults[addresses[0]]; exists {
					result[key] = status
				} else {
					result[key] = &Status{
						Online:  false,
						Latency: 0,
					}
				}
			} else {
				result[key] = &Status{
					Online:  false,
					Latency: 0,
				}
			}
		}
	}

	return result
}

// EnhancedAvailabilityTest performs availability test with consul resolution support
// Deprecated: Use TestConsulTargets for consul targets and AvailabilityTest for regular targets
func EnhancedAvailabilityTest(targets []ProxyTarget) map[string]*Status {
	result := make(map[string]*Status)

	// Group targets by type
	consulTargets := make([]ProxyTarget, 0)
	regularTargets := make([]string, 0)

	for _, target := range targets {
		if target.IsConsul && target.Resolver != "" {
			consulTargets = append(consulTargets, target)
		} else {
			// Regular target - use existing format for traditional AvailabilityTest
			key := target.Host + ":" + target.Port
			regularTargets = append(regularTargets, key)
		}
	}

	// Use traditional AvailabilityTest for regular targets (more efficient)
	if len(regularTargets) > 0 {
		regularResults := AvailabilityTest(regularTargets)
		// Merge results
		for k, v := range regularResults {
			result[k] = v
		}
	}

	// Test consul targets with DNS resolution
	if len(consulTargets) > 0 {
		consulResults := TestConsulTargets(consulTargets)
		for k, v := range consulResults {
			result[k] = v
		}
	}

	return result
}
