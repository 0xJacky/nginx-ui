package upstream

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
)

// ProxyTarget represents a proxy destination
type ProxyTarget struct {
	Host string `json:"host"`
	Port string `json:"port"`
	Type string `json:"type"` // "proxy_pass" or "upstream"
}

// ParseProxyTargetsFromRawContent parses proxy targets from raw nginx configuration content
func ParseProxyTargetsFromRawContent(content string) []ProxyTarget {
	var targets []ProxyTarget

	// First, collect all upstream names
	upstreamNames := make(map[string]bool)
	upstreamRegex := regexp.MustCompile(`(?s)upstream\s+([^\s]+)\s*\{([^}]+)\}`)
	upstreamMatches := upstreamRegex.FindAllStringSubmatch(content, -1)

	// Parse upstream blocks and collect upstream names
	for _, match := range upstreamMatches {
		if len(match) >= 3 {
			upstreamName := match[1]
			upstreamNames[upstreamName] = true

			upstreamContent := match[2]
			serverRegex := regexp.MustCompile(`(?m)^\s*server\s+([^;]+);`)
			serverMatches := serverRegex.FindAllStringSubmatch(upstreamContent, -1)

			for _, serverMatch := range serverMatches {
				if len(serverMatch) >= 2 {
					target := parseServerAddress(strings.TrimSpace(serverMatch[1]), "upstream")
					if target.Host != "" {
						targets = append(targets, target)
					}
				}
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
				target := parseProxyPassURL(proxyPassURL)
				if target.Host != "" {
					targets = append(targets, target)
				}
			}
		}
	}

	return deduplicateTargets(targets)
}

// parseUpstreamServers extracts server addresses from upstream blocks
func parseUpstreamServers(upstream *nginx.NgxUpstream) []ProxyTarget {
	var targets []ProxyTarget

	for _, directive := range upstream.Directives {
		if directive.Directive == "server" {
			target := parseServerAddress(directive.Params, "upstream")
			if target.Host != "" {
				targets = append(targets, target)
			}
		}
	}

	return targets
}

// parseLocationProxyPass extracts proxy_pass from location content
func parseLocationProxyPass(content string) []ProxyTarget {
	var targets []ProxyTarget

	// Use regex to find proxy_pass directives
	proxyPassRegex := regexp.MustCompile(`(?m)^\s*proxy_pass\s+([^;]+);`)
	matches := proxyPassRegex.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			target := parseProxyPassURL(strings.TrimSpace(match[1]))
			if target.Host != "" {
				targets = append(targets, target)
			}
		}
	}

	return targets
}

// parseProxyPassURL parses a proxy_pass URL and extracts host and port
func parseProxyPassURL(proxyPass string) ProxyTarget {
	proxyPass = strings.TrimSpace(proxyPass)

	// Handle HTTP/HTTPS URLs (e.g., "http://backend")
	if strings.HasPrefix(proxyPass, "http://") || strings.HasPrefix(proxyPass, "https://") {
		// Handle URLs with nginx variables by extracting the base URL before variables
		baseURL := proxyPass
		if dollarIndex := strings.Index(proxyPass, "$"); dollarIndex != -1 {
			baseURL = proxyPass[:dollarIndex]
		}

		if parsedURL, err := url.Parse(baseURL); err == nil {
			host := parsedURL.Hostname()
			port := parsedURL.Port()

			// Set default ports if not specified
			if port == "" {
				if parsedURL.Scheme == "https" {
					port = "443"
				} else {
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
				Type: "proxy_pass",
			}
		}
	}

	// Handle direct address format for stream module (e.g., "127.0.0.1:8080", "backend.example.com:12345")
	// This is used in stream configurations where proxy_pass doesn't require a protocol
	if !strings.Contains(proxyPass, "://") {
		target := parseServerAddress(proxyPass, "proxy_pass")

		// Skip if this is the HTTP challenge port used by Let's Encrypt
		if target.Host == "127.0.0.1" && target.Port == settings.CertSettings.HTTPChallengePort {
			return ProxyTarget{}
		}

		return target
	}

	return ProxyTarget{}
}

// parseServerAddress parses upstream server address
func parseServerAddress(serverAddr string, targetType string) ProxyTarget {
	serverAddr = strings.TrimSpace(serverAddr)

	// Remove additional parameters (weight, max_fails, etc.)
	parts := strings.Fields(serverAddr)
	if len(parts) == 0 {
		return ProxyTarget{}
	}

	addr := parts[0]

	// Handle IPv6 addresses
	if strings.HasPrefix(addr, "[") {
		// IPv6 format: [::1]:8080
		if idx := strings.LastIndex(addr, "]:"); idx != -1 {
			host := addr[1:idx]
			port := addr[idx+2:]

			// Skip if this is the HTTP challenge port used by Let's Encrypt
			if host == "::1" && port == settings.CertSettings.HTTPChallengePort {
				return ProxyTarget{}
			}

			return ProxyTarget{
				Host: host,
				Port: port,
				Type: targetType,
			}
		}
		// IPv6 without port: [::1]
		host := strings.Trim(addr, "[]")
		return ProxyTarget{
			Host: host,
			Port: "80",
			Type: targetType,
		}
	}

	// Handle IPv4 addresses and hostnames
	if strings.Contains(addr, ":") {
		parts := strings.Split(addr, ":")
		if len(parts) == 2 {
			// Skip if this is the HTTP challenge port used by Let's Encrypt
			if parts[0] == "127.0.0.1" && parts[1] == settings.CertSettings.HTTPChallengePort {
				return ProxyTarget{}
			}

			return ProxyTarget{
				Host: parts[0],
				Port: parts[1],
				Type: targetType,
			}
		}
	}

	// No port specified, use default
	return ProxyTarget{
		Host: addr,
		Port: "80",
		Type: targetType,
	}
}

// deduplicateTargets removes duplicate proxy targets
func deduplicateTargets(targets []ProxyTarget) []ProxyTarget {
	seen := make(map[string]bool)
	var result []ProxyTarget

	for _, target := range targets {
		key := target.Host + ":" + target.Port + ":" + target.Type
		if !seen[key] {
			seen[key] = true
			result = append(result, target)
		}
	}

	return result
}

// isUpstreamReference checks if a proxy_pass URL references an upstream block
func isUpstreamReference(proxyPass string, upstreamNames map[string]bool) bool {
	proxyPass = strings.TrimSpace(proxyPass)

	// For HTTP/HTTPS URLs, parse the URL to extract the hostname
	if strings.HasPrefix(proxyPass, "http://") || strings.HasPrefix(proxyPass, "https://") {
		// Handle URLs with nginx variables (e.g., "https://myUpStr$request_uri")
		// Extract the scheme and hostname part before any nginx variables
		schemeAndHost := proxyPass
		if dollarIndex := strings.Index(proxyPass, "$"); dollarIndex != -1 {
			schemeAndHost = proxyPass[:dollarIndex]
		}

		// Try to parse the URL, if it fails, try manual extraction
		if parsedURL, err := url.Parse(schemeAndHost); err == nil {
			hostname := parsedURL.Hostname()
			// Check if the hostname matches any upstream name
			return upstreamNames[hostname]
		} else {
			// Fallback: manually extract hostname for URLs with variables
			// Remove scheme prefix
			withoutScheme := proxyPass
			if strings.HasPrefix(proxyPass, "https://") {
				withoutScheme = strings.TrimPrefix(proxyPass, "https://")
			} else if strings.HasPrefix(proxyPass, "http://") {
				withoutScheme = strings.TrimPrefix(proxyPass, "http://")
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

	// For stream module, proxy_pass can directly reference upstream name without protocol
	// Check if the proxy_pass value directly matches an upstream name
	if !strings.Contains(proxyPass, "://") && !strings.Contains(proxyPass, ":") {
		return upstreamNames[proxyPass]
	}

	return false
}
