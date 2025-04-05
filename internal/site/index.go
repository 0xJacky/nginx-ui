package site

import (
	"net"
	"path/filepath"
	"regexp"
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

	// Map to track hosts and their SSL status
	hostMap := make(map[string]bool)

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

		// Check if SSL is enabled
		listenMatches := listenRegex.FindAllSubmatch(serverBlockContent, -1)
		hasSSL := false

		for _, match := range listenMatches {
			if len(match) >= 2 {
				listenValue := string(match[1])
				if strings.Contains(listenValue, "ssl") || strings.Contains(listenValue, "443") {
					hasSSL = true
					break
				}
			}
		}

		// Update host map with SSL status
		for _, name := range validServerNames {
			// Only update if this host doesn't have SSL yet
			if currentSSL, exists := hostMap[name]; !exists || !currentSSL {
				hostMap[name] = hasSSL
			}
		}
	}

	// Generate URLs from the host map
	for host, hasSSL := range hostMap {
		protocol := "http"
		if hasSSL {
			protocol = "https"
		}
		url := protocol + "://" + host
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
