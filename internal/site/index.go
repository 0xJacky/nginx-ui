package site

import (
	"net"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
)

type SiteIndex struct {
	Path         string
	Content      string
	Urls         []string
	ProxyTargets []ProxyTarget
}

var (
	IndexedSites = make(map[string]*SiteIndex)
)

func GetIndexedSite(path string) *SiteIndex {
	if site, ok := IndexedSites[path]; ok {
		return site
	}
	return &SiteIndex{}
}

func init() {
	cache.RegisterCallback(scanForSite)
}

func scanForSite(configPath string, content []byte) error {
	// Regular expressions for server_name and listen directives
	serverNameRegex := regexp.MustCompile(`(?m)server_name\s+([^;]+);`)
	listenRegex := regexp.MustCompile(`(?m)listen\s+([^;]+);`)
	returnRegex := regexp.MustCompile(`(?m)return\s+30[1-8]\s+https://`)

	// Find server blocks
	serverBlockRegex := regexp.MustCompile(`(?ms)server\s*\{[^\{]*((.*?\{.*?\})*?[^\}]*)\}`)
	serverBlocks := serverBlockRegex.FindAllSubmatch(content, -1)

	siteIndex := SiteIndex{
		Path:         configPath,
		Content:      string(content),
		Urls:         []string{},
		ProxyTargets: []ProxyTarget{},
	}

	// Map to track hosts, their SSL status and port
	type hostInfo struct {
		hasSSL      bool
		port        int
		isPublic    bool // Whether this is a public-facing port
		priority    int  // Higher priority for public ports
		hasRedirect bool // Whether this server block has HTTPS redirect
	}
	hostMap := make(map[string]hostInfo)

	for _, block := range serverBlocks {
		serverBlockContent := block[0]

		// Extract server_name values
		serverNameMatches := serverNameRegex.FindSubmatch(serverBlockContent)
		if len(serverNameMatches) < 2 {
			continue
		}

		// Get all server names
		serverNames := strings.Fields(string(serverNameMatches[1]))
		var validServerNames []string

		// Filter valid domain names and IPs
		for _, name := range serverNames {
			// Skip placeholder names
			if name == "_" || name == "localhost" {
				continue
			}

			// Check if it's a valid IP
			if net.ParseIP(name) != nil {
				validServerNames = append(validServerNames, name)
				continue
			}

			// Basic domain validation
			if isValidDomain(name) {
				validServerNames = append(validServerNames, name)
			}
		}

		if len(validServerNames) == 0 {
			continue
		}

		// Check if this server block has HTTPS redirect
		hasRedirect := returnRegex.Match(serverBlockContent)

		// Check if SSL is enabled and extract port
		listenMatches := listenRegex.FindAllSubmatch(serverBlockContent, -1)

		for _, match := range listenMatches {
			if len(match) >= 2 {
				listenValue := strings.TrimSpace(string(match[1]))
				hasSSL := strings.Contains(listenValue, "ssl")
				port := 80 // Default HTTP port
				isPublic := true
				priority := 1

				if hasSSL {
					port = 443   // Default HTTPS port
					priority = 3 // SSL has highest priority
				} else if hasRedirect {
					priority = 2 // HTTP with redirect has medium priority
				}

				// Parse different listen directive formats
				// Format examples:
				// - 80
				// - 443 ssl
				// - [::]:80
				// - 127.0.0.1:8443 ssl
				// - *:80

				// Remove extra parameters (ssl, http2, etc.) for parsing
				listenParts := strings.Fields(listenValue)
				if len(listenParts) > 0 {
					addressPart := listenParts[0]

					// Check if it's bound to a specific IP (not public)
					if strings.Contains(addressPart, "127.0.0.1") ||
						strings.Contains(addressPart, "localhost") {
						isPublic = false
						priority = 0 // Internal ports have lowest priority
					}

					// Extract port from various formats
					var extractedPort int
					var err error

					if strings.Contains(addressPart, ":") {
						// Handle IPv6 format [::]:port or IPv4 format ip:port
						if strings.HasPrefix(addressPart, "[") {
							// IPv6 format: [::]:80
							if colonIndex := strings.LastIndex(addressPart, ":"); colonIndex != -1 {
								portStr := addressPart[colonIndex+1:]
								extractedPort, err = strconv.Atoi(portStr)
							}
						} else {
							// IPv4 format: 127.0.0.1:8443 or *:80
							if colonIndex := strings.LastIndex(addressPart, ":"); colonIndex != -1 {
								portStr := addressPart[colonIndex+1:]
								extractedPort, err = strconv.Atoi(portStr)
							}
						}
					} else {
						// Just a port number: 80, 443
						extractedPort, err = strconv.Atoi(addressPart)
					}

					if err == nil && extractedPort > 0 {
						port = extractedPort
					}
				}

				// Update host map with SSL status and port, prioritizing public ports
				for _, name := range validServerNames {
					info, exists := hostMap[name]

					// Update if:
					// 1. Host doesn't exist yet
					// 2. New entry has higher priority (SSL > redirect > plain HTTP, public > private)
					// 3. Same priority but adding SSL
					if !exists ||
						priority > info.priority ||
						(priority == info.priority && hasSSL && !info.hasSSL) {
						hostMap[name] = hostInfo{
							hasSSL:      hasSSL,
							port:        port,
							isPublic:    isPublic,
							priority:    priority,
							hasRedirect: hasRedirect,
						}
					}
				}
			}
		}
	}

	// Generate URLs from the host map
	for host, info := range hostMap {
		// Skip internal/private addresses for URL generation
		if !info.isPublic {
			continue
		}

		protocol := "http"
		defaultPort := 80

		// If we have a redirect to HTTPS, prefer HTTPS URL
		if info.hasSSL || info.hasRedirect {
			protocol = "https"
			defaultPort = 443
		}

		url := protocol + "://" + host

		// Add port to URL if non-standard
		if info.port != defaultPort && info.hasSSL {
			// Only add port for SSL if it's not the default SSL port
			url += ":" + strconv.Itoa(info.port)
		} else if info.port != defaultPort && !info.hasSSL && !info.hasRedirect {
			// Add port for non-SSL, non-redirect cases
			url += ":" + strconv.Itoa(info.port)
		}

		siteIndex.Urls = append(siteIndex.Urls, url)
	}

	// Parse proxy targets from the configuration content
	siteIndex.ProxyTargets = upstream.ParseProxyTargetsFromRawContent(string(content))

	// Only store if we found valid URLs or proxy targets
	if len(siteIndex.Urls) > 0 || len(siteIndex.ProxyTargets) > 0 {
		IndexedSites[filepath.Base(configPath)] = &siteIndex
	}

	return nil
}

// isValidDomain performs a basic validation of domain names
func isValidDomain(domain string) bool {
	// Basic validation: contains at least one dot and no spaces
	return strings.Contains(domain, ".") && !strings.Contains(domain, " ")
}
