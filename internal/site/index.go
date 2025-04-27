package site

import (
	"net"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/cache"
)

type SiteIndex struct {
	Path    string
	Content string
	Urls    []string
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

	// Find server blocks
	serverBlockRegex := regexp.MustCompile(`(?ms)server\s*\{[^\{]*((.*?\{.*?\})*?[^\}]*)\}`)
	serverBlocks := serverBlockRegex.FindAllSubmatch(content, -1)

	siteIndex := SiteIndex{
		Path:    configPath,
		Content: string(content),
		Urls:    []string{},
	}

	// Map to track hosts, their SSL status and port
	type hostInfo struct {
		hasSSL bool
		port   int
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

		// Check if SSL is enabled and extract port
		listenMatches := listenRegex.FindAllSubmatch(serverBlockContent, -1)
		hasSSL := false
		port := 80 // Default HTTP port

		for _, match := range listenMatches {
			if len(match) >= 2 {
				listenValue := string(match[1])
				if strings.Contains(listenValue, "ssl") {
					hasSSL = true
					port = 443 // Default HTTPS port
				}

				// Extract port number if present
				portRegex := regexp.MustCompile(`^(?:(\d+)|.*:(\d+))`)
				portMatches := portRegex.FindStringSubmatch(listenValue)
				if len(portMatches) > 0 {
					// Check which capture group has the port
					portStr := ""
					if portMatches[1] != "" {
						portStr = portMatches[1]
					} else if portMatches[2] != "" {
						portStr = portMatches[2]
					}

					if portStr != "" {
						if extractedPort, err := strconv.Atoi(portStr); err == nil {
							port = extractedPort
						}
					}
				}
			}
		}

		// Update host map with SSL status and port
		for _, name := range validServerNames {
			// Only update if this host doesn't have SSL yet or we're adding SSL now
			info, exists := hostMap[name]
			if !exists || (!info.hasSSL && hasSSL) {
				hostMap[name] = hostInfo{hasSSL: hasSSL, port: port}
			}
		}
	}

	// Generate URLs from the host map
	for host, info := range hostMap {
		protocol := "http"
		defaultPort := 80

		if info.hasSSL {
			protocol = "https"
			defaultPort = 443
		}

		url := protocol + "://" + host

		// Add port to URL if non-standard
		if info.port != defaultPort {
			url += ":" + strconv.Itoa(info.port)
		}

		siteIndex.Urls = append(siteIndex.Urls, url)
	}

	// Only store if we found valid URLs
	if len(siteIndex.Urls) > 0 {
		IndexedSites[filepath.Base(configPath)] = &siteIndex
	}

	return nil
}

// isValidDomain performs a basic validation of domain names
func isValidDomain(domain string) bool {
	// Basic validation: contains at least one dot and no spaces
	return strings.Contains(domain, ".") && !strings.Contains(domain, " ")
}
