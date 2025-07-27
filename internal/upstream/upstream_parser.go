package upstream

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
)

// ProxyTarget represents a proxy destination
type ProxyTarget struct {
	Host       string `json:"host"`
	Port       string `json:"port"`
	Type       string `json:"type"`        // "proxy_pass", "grpc_pass" or "upstream"
	Resolver   string `json:"resolver"`    // DNS resolver address (e.g., "127.0.0.1:8600")
	IsConsul   bool   `json:"is_consul"`   // Whether this is a consul service discovery target
	ServiceURL string `json:"service_url"` // Full service URL for consul (e.g., "service.consul service=redacted-net resolve")
}

// UpstreamContext contains upstream-level configuration
type UpstreamContext struct {
	Name     string
	Resolver string
}

// ParseResult contains the results of parsing nginx configuration
type ParseResult struct {
	ProxyTargets []ProxyTarget
	Upstreams    map[string][]ProxyTarget // upstream name -> servers
}

// ParseProxyTargetsFromRawContent parses proxy targets from raw nginx configuration content
func ParseProxyTargetsFromRawContent(content string) []ProxyTarget {
	result := ParseProxyTargetsAndUpstreamsFromRawContent(content)
	return result.ProxyTargets
}

// ParseProxyTargetsAndUpstreamsFromRawContent parses both proxy targets and upstream definitions from raw nginx configuration content
func ParseProxyTargetsAndUpstreamsFromRawContent(content string) *ParseResult {
	var targets []ProxyTarget
	upstreams := make(map[string][]ProxyTarget)

	// First, collect all upstream names and their contexts
	upstreamNames := make(map[string]bool)
	upstreamContexts := make(map[string]*UpstreamContext)
	upstreamRegex := regexp.MustCompile(`(?s)upstream\s+([^\s]+)\s*\{([^}]+)\}`)
	upstreamMatches := upstreamRegex.FindAllStringSubmatch(content, -1)

	// Parse upstream blocks and collect upstream names
	for _, match := range upstreamMatches {
		if len(match) >= 3 {
			upstreamName := match[1]
			upstreamNames[upstreamName] = true
			upstreamContent := match[2]

			// Create upstream context
			ctx := &UpstreamContext{
				Name: upstreamName,
			}

			// Extract resolver information from upstream block
			resolverRegex := regexp.MustCompile(`(?m)^\s*resolver\s+([^;]+);`)
			if resolverMatch := resolverRegex.FindStringSubmatch(upstreamContent); len(resolverMatch) >= 2 {
				// Parse resolver directive (e.g., "127.0.0.1:8600 valid=5s ipv6=off")
				resolverParts := strings.Fields(resolverMatch[1])
				if len(resolverParts) > 0 {
					ctx.Resolver = resolverParts[0] // Take the first part as resolver address
				}
			}

			upstreamContexts[upstreamName] = ctx

			serverRegex := regexp.MustCompile(`(?m)^\s*server\s+([^;]+);`)
			serverMatches := serverRegex.FindAllStringSubmatch(upstreamContent, -1)

			var upstreamServers []ProxyTarget
			for _, serverMatch := range serverMatches {
				if len(serverMatch) >= 2 {
					target := parseServerAddress(strings.TrimSpace(serverMatch[1]), "upstream", ctx)
					if target.Host != "" {
						targets = append(targets, target)
						upstreamServers = append(upstreamServers, target)
					}
				}
			}

			// Store upstream definition
			if len(upstreamServers) > 0 {
				upstreams[upstreamName] = upstreamServers
			}
		}
	}

	// Parse proxy_pass directives, but skip upstream references
	proxyPassRegex := regexp.MustCompile(`(?m)^\s*proxy_pass\s+([^;]+);`)
	proxyMatches := proxyPassRegex.FindAllStringSubmatch(content, -1)

	for _, match := range proxyMatches {
		if len(match) >= 2 {
			proxyPassURL := strings.TrimSpace(match[1])
			// Skip if this proxy_pass references an upstream
			if !isUpstreamReference(proxyPassURL, upstreamNames) {
				target := parseProxyPassURL(proxyPassURL, "proxy_pass")
				if target.Host != "" {
					targets = append(targets, target)
				}
			}
		}
	}

	// Parse grpc_pass directives, but skip upstream references
	grpcPassRegex := regexp.MustCompile(`(?m)^\s*grpc_pass\s+([^;]+);`)
	grpcMatches := grpcPassRegex.FindAllStringSubmatch(content, -1)

	for _, match := range grpcMatches {
		if len(match) >= 2 {
			grpcPassURL := strings.TrimSpace(match[1])
			// Skip if this grpc_pass references an upstream
			if !isUpstreamReference(grpcPassURL, upstreamNames) {
				target := parseProxyPassURL(grpcPassURL, "grpc_pass")
				if target.Host != "" {
					targets = append(targets, target)
				}
			}
		}
	}

	return &ParseResult{
		ProxyTargets: deduplicateTargets(targets),
		Upstreams:    upstreams,
	}
}

// parseProxyPassURL parses a proxy_pass or grpc_pass URL and extracts host and port
func parseProxyPassURL(passURL, passType string) ProxyTarget {
	passURL = strings.TrimSpace(passURL)

	// Skip URLs that contain Nginx variables
	if strings.Contains(passURL, "$") {
		return ProxyTarget{}
	}

	// Handle HTTP/HTTPS/gRPC URLs (e.g., "http://backend", "grpc://backend")
	if strings.HasPrefix(passURL, "http://") || strings.HasPrefix(passURL, "https://") || strings.HasPrefix(passURL, "grpc://") || strings.HasPrefix(passURL, "grpcs://") {
		if parsedURL, err := url.Parse(passURL); err == nil {
			host := parsedURL.Hostname()
			port := parsedURL.Port()

			// Set default ports if not specified
			if port == "" {
				switch parsedURL.Scheme {
				case "https":
					port = "443"
				case "grpcs":
					port = "443"
				case "grpc":
					port = "80"
				default: // http
					port = "80"
				}
			}

			// Skip if this is the HTTP challenge port used by Let's Encrypt
			if host == "127.0.0.1" && port == settings.CertSettings.HTTPChallengePort {
				return ProxyTarget{}
			}

			return ProxyTarget{
				Host: host,
				Port: port,
				Type: passType,
			}
		}
	}

	// Handle direct address format for stream module (e.g., "127.0.0.1:8080", "backend.example.com:12345")
	// This is used in stream configurations where proxy_pass/grpc_pass doesn't require a protocol
	if !strings.Contains(passURL, "://") {
		target := parseServerAddress(passURL, passType, nil) // No upstream context for this function

		// Skip if this is the HTTP challenge port used by Let's Encrypt
		if target.Host == "127.0.0.1" && target.Port == settings.CertSettings.HTTPChallengePort {
			return ProxyTarget{}
		}

		return target
	}

	return ProxyTarget{}
}

// parseServerAddress parses upstream server address with upstream context
func parseServerAddress(serverAddr string, targetType string, ctx *UpstreamContext) ProxyTarget {
	serverAddr = strings.TrimSpace(serverAddr)

	// Remove additional parameters (weight, max_fails, etc.)
	parts := strings.Fields(serverAddr)
	if len(parts) == 0 {
		return ProxyTarget{}
	}

	addr := parts[0]
	target := ProxyTarget{
		Type: targetType,
	}

	// Add resolver information from upstream context
	if ctx != nil && ctx.Resolver != "" {
		target.Resolver = ctx.Resolver
	}

	// Check if the address contains Nginx variables - skip if it does
	if strings.Contains(addr, "$") {
		return ProxyTarget{}
	}

	// Check for consul service discovery patterns
	if isConsulServiceDiscovery(serverAddr) {
		target.IsConsul = true
		target.ServiceURL = serverAddr

		// Extract consul DNS host (e.g., "service.consul")
		if strings.Contains(addr, "service.consul") {
			target.Host = "service.consul"
			// For consul service discovery, we use a placeholder port since the actual port is dynamic
			target.Port = "dynamic"
		} else {
			// Fallback to regular parsing
			parsed := parseAddressOnly(addr)
			target.Host = parsed.Host
			target.Port = parsed.Port
		}

		return target
	}

	// Regular address parsing
	parsed := parseAddressOnly(addr)
	target.Host = parsed.Host
	target.Port = parsed.Port

	// Skip if this is the HTTP challenge port used by Let's Encrypt
	if target.Host == "127.0.0.1" && target.Port == settings.CertSettings.HTTPChallengePort {
		return ProxyTarget{}
	}

	return target
}

// isConsulServiceDiscovery checks if the server address is a dynamic service discovery configuration
// This includes both Consul and standard nginx service= configurations
func isConsulServiceDiscovery(serverAddr string) bool {
	// Standard nginx service= format: "hostname service=name resolve"
	if strings.Contains(serverAddr, "service=") && strings.Contains(serverAddr, "resolve") {
		return true
	}
	// Legacy consul format: "service.consul service=name resolve"
	return strings.Contains(serverAddr, "service.consul") &&
		(strings.Contains(serverAddr, "service=") || strings.Contains(serverAddr, "resolve"))
}

// parseAddressOnly parses just the address portion without consul-specific logic
// Supports both IPv4 and IPv6 addresses
func parseAddressOnly(addr string) ProxyTarget {
	// Handle IPv6 addresses with brackets
	if strings.HasPrefix(addr, "[") {
		// IPv6 format: [::1]:8080 or [2001:db8::1]:8080
		if idx := strings.LastIndex(addr, "]:"); idx != -1 {
			host := addr[1:idx]
			port := addr[idx+2:]
			return ProxyTarget{
				Host: host,
				Port: port,
			}
		}
		// IPv6 without port: [::1] or [2001:db8::1]
		host := strings.Trim(addr, "[]")
		return ProxyTarget{
			Host: host,
			Port: "80",
		}
	}

	// Check if this might be an IPv6 address without brackets
	// IPv6 addresses contain multiple colons
	colonCount := strings.Count(addr, ":")
	if colonCount > 1 {
		// This is likely an IPv6 address without brackets and without port
		// e.g., ::1, 2001:db8::1, fe80::1%eth0
		return ProxyTarget{
			Host: addr,
			Port: "80",
		}
	}

	// Handle IPv4 addresses and hostnames with port
	if strings.Contains(addr, ":") {
		parts := strings.Split(addr, ":")
		if len(parts) == 2 {
			return ProxyTarget{
				Host: parts[0],
				Port: parts[1],
			}
		}
	}

	// No port specified, use default
	return ProxyTarget{
		Host: addr,
		Port: "80",
	}
}

// deduplicateTargets removes duplicate proxy targets
func deduplicateTargets(targets []ProxyTarget) []ProxyTarget {
	seen := make(map[string]bool)
	var result []ProxyTarget

	for _, target := range targets {
		// Create a unique key that includes resolver and consul information
		key := target.Host + ":" + target.Port + ":" + target.Type + ":" + target.Resolver
		if target.IsConsul {
			key += ":consul:" + target.ServiceURL
		}

		if !seen[key] {
			seen[key] = true
			result = append(result, target)
		}
	}

	return result
}

// isUpstreamReference checks if a proxy_pass or grpc_pass URL references an upstream block
func isUpstreamReference(passURL string, upstreamNames map[string]bool) bool {
	passURL = strings.TrimSpace(passURL)

	// For HTTP/HTTPS/gRPC URLs, parse the URL to extract the hostname
	if strings.HasPrefix(passURL, "http://") || strings.HasPrefix(passURL, "https://") || strings.HasPrefix(passURL, "grpc://") || strings.HasPrefix(passURL, "grpcs://") {
		// Handle URLs with nginx variables (e.g., "https://myUpStr$request_uri")
		// Extract the scheme and hostname part before any nginx variables
		schemeAndHost := passURL
		if dollarIndex := strings.Index(passURL, "$"); dollarIndex != -1 {
			schemeAndHost = passURL[:dollarIndex]
		}

		// Try to parse the URL, if it fails, try manual extraction
		if parsedURL, err := url.Parse(schemeAndHost); err == nil {
			hostname := parsedURL.Hostname()
			// Check if the hostname matches any upstream name
			return upstreamNames[hostname]
		} else {
			// Fallback: manually extract hostname for URLs with variables
			// Remove scheme prefix
			withoutScheme := passURL
			if strings.HasPrefix(passURL, "https://") {
				withoutScheme = strings.TrimPrefix(passURL, "https://")
			} else if strings.HasPrefix(passURL, "http://") {
				withoutScheme = strings.TrimPrefix(passURL, "http://")
			} else if strings.HasPrefix(passURL, "grpc://") {
				withoutScheme = strings.TrimPrefix(passURL, "grpc://")
			} else if strings.HasPrefix(passURL, "grpcs://") {
				withoutScheme = strings.TrimPrefix(passURL, "grpcs://")
			}

			// Extract hostname before any path, port, or variable
			hostname := withoutScheme
			if slashIndex := strings.Index(hostname, "/"); slashIndex != -1 {
				hostname = hostname[:slashIndex]
			}
			if colonIndex := strings.Index(hostname, ":"); colonIndex != -1 {
				hostname = hostname[:colonIndex]
			}
			if dollarIndex := strings.Index(hostname, "$"); dollarIndex != -1 {
				hostname = hostname[:dollarIndex]
			}

			return upstreamNames[hostname]
		}
	}

	// For stream module, proxy_pass/grpc_pass can directly reference upstream name without protocol
	// Check if the pass value directly matches an upstream name
	if !strings.Contains(passURL, "://") && !strings.Contains(passURL, ":") {
		return upstreamNames[passURL]
	}

	return false
}
