package site

import (
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/uozi-tech/cosy/logger"
)

type Index struct {
	Path         string
	Content      string
	Urls         []string
	ProxyTargets []ProxyTarget
}

var (
	IndexedSites   = make(map[string]*Index)
	siteIndexMutex sync.RWMutex
)

func GetIndexedSite(path string) *Index {
	siteIndexMutex.RLock()
	defer siteIndexMutex.RUnlock()

	if site, ok := IndexedSites[path]; ok {
		return site
	}
	return &Index{}
}

func init() {
	cache.RegisterCallback("site.scanForSite", scanForSite)
}

func scanForSite(configPath string, content []byte) error {
	// Handle file removal - clean up the index entry
	if len(content) == 0 {
		siteIndexMutex.Lock()
		delete(IndexedSites, filepath.Base(configPath))
		siteIndexMutex.Unlock()
		return nil
	}

	// Regular expressions for server_name and listen directives
	serverNameRegex := regexp.MustCompile(`(?m)^[ \t]*server_name\s+([^;#]+);`)
	listenRegex := regexp.MustCompile(`(?m)^[ \t]*listen\s+([^;#]+);`)
	includeRegex := regexp.MustCompile(`(?m)^[ \t]*include\s+([^;#]+);`)
	returnRegex := regexp.MustCompile(`(?m)^[ \t]*return\s+30[1-8]\s+https://[^\s;#]+`)

	baseDir := filepath.Dir(configPath)
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		logger.Debugf("Failed to resolve base path for config %s: %v", configPath, err)
		return err
	}

	allowedRoot := baseAbs
	if confRoot := nginx.GetConfPath(); confRoot != "" {
		confRootAbs, err := filepath.Abs(confRoot)
		if err != nil {
			logger.Debugf("Failed to resolve nginx conf root %s: %v", confRoot, err)
		} else {
			allowedRoot = confRootAbs
		}
	}

	// Find server blocks
	serverBlockRegex := regexp.MustCompile(`(?ms)server\s*\{[^\{]*((.*?\{.*?\})*?[^\}]*)\}`)
	serverBlocks := serverBlockRegex.FindAllSubmatch(content, -1)

	siteIndex := Index{
		Path:         configPath,
		Content:      string(content),
		Urls:         []string{},
		ProxyTargets: []ProxyTarget{},
	}

	// Map to track hosts, their SSL status and port
	type hostInfo struct {
		hasSSL      bool
		port        int
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

		// Collect listen directives from this server block and its includes
		listenMatches := listenRegex.FindAllSubmatch(serverBlockContent, -1)

		// Parse includes inside the server block (common pattern: listen is placed in snippet)
		includeMatches := includeRegex.FindAllSubmatch(serverBlockContent, -1)
		for _, match := range includeMatches {
			if len(match) < 2 {
				continue
			}
			includePath := strings.TrimSpace(string(match[1]))
			includePath = strings.Trim(includePath, `"'`)
			// Skip wildcard includes for now; they can represent multiple files
			if strings.Contains(includePath, "*") {
				logger.Debugf("Skipping wildcard include in site scan: %s", includePath)
				continue
			}

			// Resolve relative paths: prefer same directory; fallback to nginx root (allowedRoot)
			if !filepath.IsAbs(includePath) {
				candidate := filepath.Join(baseDir, includePath)
				if _, err := os.Stat(candidate); err != nil {
					altCandidate := filepath.Join(allowedRoot, includePath)
					if _, altErr := os.Stat(altCandidate); altErr == nil {
						candidate = altCandidate
					}
				}
				includePath = candidate
			}

			includePath = filepath.Clean(includePath)
			includeAbs, err := filepath.Abs(includePath)
			if err != nil {
				logger.Debugf("Failed to resolve absolute path for include during site scan: %s, error: %v", includePath, err)
				continue
			}

			rel, err := filepath.Rel(allowedRoot, includeAbs)
			if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
				logger.Debugf("Blocked include outside base config directory during site scan: %s", includeAbs)
				continue
			}

			includeContent, err := os.ReadFile(includeAbs)
			if err != nil {
				logger.Debugf("Failed to read include file during site scan: %s, error: %v", includeAbs, err)
				continue
			}

			includeListens := listenRegex.FindAllSubmatch(includeContent, -1)
			if len(includeListens) == 0 {
				logger.Debugf("No listen directives found in include file during site scan: %s", includePath)
				continue
			}
			listenMatches = append(listenMatches, includeListens...)
		}

		// If no listen directives were found, assume default HTTP :80 to keep site visible
		if len(listenMatches) == 0 {
			logger.Debugf("No listen directive found for server block in %s, defaulting to :80 http", configPath)
			listenMatches = append(listenMatches, [][]byte{{}, []byte("80")})
		}

		for _, match := range listenMatches {
			if len(match) >= 2 {
				listenValue := strings.TrimSpace(string(match[1]))
				hasSSL := strings.Contains(listenValue, "ssl")
				port := 80 // Default HTTP port
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
		siteIndexMutex.Lock()
		IndexedSites[filepath.Base(configPath)] = &siteIndex
		siteIndexMutex.Unlock()
	}

	return nil
}

// isValidDomain performs a basic validation of domain names
func isValidDomain(domain string) bool {
	// Basic validation: contains at least one dot and no spaces
	return strings.Contains(domain, ".") && !strings.Contains(domain, " ")
}
