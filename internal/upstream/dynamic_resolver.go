package upstream

// Package upstream provides DNS resolution and availability testing for nginx upstream targets,
// with special support for dynamic service discovery and nginx-style SRV record resolution.
//
// # SRV Record Resolution (nginx.org compliant)
//
// This package implements nginx's SRV record resolution rules as documented at nginx.org.
// The service=name parameter enables resolving of DNS SRV records and sets the service name.
//
// Rules for SRV record construction:
//
//  1. If the service name does not contain a dot ("."), then the RFC-compliant name is constructed
//     and the TCP protocol is added to the service prefix.
//     Example: "backend.example.com service=http resolve" -> "_http._tcp.backend.example.com"
//
//  2. If the service name contains one or more dots, then the name is constructed by joining
//     the service prefix and the server name.
//     Example: "backend.example.com service=_http._tcp resolve" -> "_http._tcp.backend.example.com"
//     Example: "example.com service=server1.backend resolve" -> "server1.backend.example.com"
//
// # Dynamic DNS Integration
//
// The resolver supports various DNS interfaces for service discovery:
// - DNS servers (e.g., Consul DNS on 127.0.0.1:8600, CoreDNS, etc.)
// - Service registration and health checking
// - SRV record-based load balancing with proper priority handling
//
// # Usage Examples
//
//	// Create resolver with DNS server
//	resolver := NewDynamicResolver("127.0.0.1:8600")
//
//	// Resolve nginx-style service URL
//	addresses, err := resolver.ResolveService("backend.example.com service=http resolve")
//	// This queries "_http._tcp.backend.example.com" SRV records
//
//	// Resolve with dotted service name
//	addresses, err := resolver.ResolveService("example.com service=server1.backend resolve")
//	// This queries "server1.backend.example.com" SRV records

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// DynamicResolver handles DNS resolution through dynamic DNS servers
type DynamicResolver struct {
	resolver string // e.g., "127.0.0.1:8600"
}

// NewDynamicResolver creates a new dynamic resolver
func NewDynamicResolver(resolver string) *DynamicResolver {
	return &DynamicResolver{
		resolver: resolver,
	}
}

// ServiceInfo contains parsed service information from nginx config
type ServiceInfo struct {
	Hostname    string // e.g., "backend.example.com" or "service.consul"
	ServiceName string // e.g., "http", "_http._tcp", "server1.backend"
}

// ResolveService resolves a nginx service to actual IP addresses and ports
func (dr *DynamicResolver) ResolveService(serviceURL string) ([]string, error) {
	// Parse service URL to extract hostname and service name
	serviceInfo, err := dr.parseServiceURL(serviceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse service URL %s: %v", serviceURL, err)
	}

	// Create a custom resolver that uses the DNS server
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, dr.resolver)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Construct SRV query domain according to nginx rules
	srvDomain := dr.constructSRVDomain(serviceInfo)

	// Query for service SRV records
	_, srvRecords, err := resolver.LookupSRV(ctx, "", "", srvDomain)
	if err != nil {
		// Fallback to A record lookup if SRV fails
		ips, err := resolver.LookupIPAddr(ctx, srvDomain)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve service %s: %v", srvDomain, err)
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
		return nil, fmt.Errorf("no addresses found for service %s", srvDomain)
	}

	return addresses, nil
}

// parseServiceURL parses nginx service URL and extracts hostname and service name
func (dr *DynamicResolver) parseServiceURL(serviceURL string) (*ServiceInfo, error) {
	serviceURL = strings.TrimSpace(serviceURL)
	if serviceURL == "" {
		return nil, fmt.Errorf("empty service URL")
	}

	// Parse nginx format: "hostname service=servicename resolve"
	parts := strings.Fields(serviceURL)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid service URL format: %s", serviceURL)
	}

	hostname := parts[0]
	var serviceName string

	// Find service=name parameter
	for _, part := range parts[1:] {
		if strings.HasPrefix(part, "service=") {
			serviceName = strings.TrimPrefix(part, "service=")
			serviceName = strings.TrimSpace(serviceName)
			break
		}
	}

	if serviceName == "" {
		return nil, fmt.Errorf("service parameter not found in: %s", serviceURL)
	}

	return &ServiceInfo{
		Hostname:    hostname,
		ServiceName: serviceName,
	}, nil
}

// constructSRVDomain constructs SRV query domain according to nginx.org rules
func (dr *DynamicResolver) constructSRVDomain(serviceInfo *ServiceInfo) string {
	// According to nginx.org documentation:
	// 1. If service name does not contain a dot ("."), then RFC-compliant name is constructed
	//    and TCP protocol is added to the service prefix.
	//    Example: service=http -> _http._tcp.hostname
	// 2. If service name contains one or more dots, then the name is constructed by joining
	//    the service prefix and the server name.
	//    Example: service=_http._tcp -> _http._tcp.hostname
	//    Example: service=server1.backend -> server1.backend.hostname

	if !strings.Contains(serviceInfo.ServiceName, ".") {
		// Case 1: No dots - construct RFC-compliant name with TCP protocol
		return fmt.Sprintf("_%s._tcp.%s", serviceInfo.ServiceName, serviceInfo.Hostname)
	} else {
		// Case 2: Contains dots - join service prefix and hostname
		return fmt.Sprintf("%s.%s", serviceInfo.ServiceName, serviceInfo.Hostname)
	}
}

// extractServiceName extracts the service name from service URL
// Deprecated: Use parseServiceURL instead for proper nginx-style parsing
func (dr *DynamicResolver) extractServiceName(serviceURL string) string {
	serviceInfo, err := dr.parseServiceURL(serviceURL)
	if err != nil {
		// Fallback to old parsing logic for backward compatibility
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

	return serviceInfo.ServiceName
}

// TestDynamicTargets performs availability test specifically for dynamic DNS targets
func TestDynamicTargets(dynamicTargets []ProxyTarget) map[string]*Status {
	result := make(map[string]*Status)

	// Group dynamic targets by resolver
	dynamicTargetsByResolver := make(map[string][]ProxyTarget)
	for _, target := range dynamicTargets {
		if target.Resolver != "" {
			dynamicTargetsByResolver[target.Resolver] = append(dynamicTargetsByResolver[target.Resolver], target)
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
	for resolver, targets := range dynamicTargetsByResolver {
		dynamicResolver := NewDynamicResolver(resolver)

		for _, target := range targets {
			key := target.Host + ":" + target.Port

			// Try to resolve the service
			addresses, err := dynamicResolver.ResolveService(target.ServiceURL)
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

// EnhancedAvailabilityTest performs availability test with dynamic DNS resolution support
// Deprecated: Use TestDynamicTargets for dynamic targets and AvailabilityTest for regular targets
func EnhancedAvailabilityTest(targets []ProxyTarget) map[string]*Status {
	result := make(map[string]*Status)

	// Group targets by type
	dynamicTargets := make([]ProxyTarget, 0)
	regularTargets := make([]string, 0)

	for _, target := range targets {
		if target.IsConsul && target.Resolver != "" {
			dynamicTargets = append(dynamicTargets, target)
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

	// Test dynamic targets with DNS resolution
	if len(dynamicTargets) > 0 {
		dynamicResults := TestDynamicTargets(dynamicTargets)
		for k, v := range dynamicResults {
			result[k] = v
		}
	}

	return result
}
