package sitecheck

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
)

// Site config cache with expiration
var (
	siteConfigCache = make(map[string]*siteConfigCacheEntry)
	siteConfigMutex sync.RWMutex
	cacheExpiry     = 5 * time.Minute // Cache entries expire after 5 minutes
	lastBatchLoad   time.Time
)

type siteConfigCacheEntry struct {
	config    *model.SiteConfig
	expiresAt time.Time
}

type SiteChecker struct {
	sites            map[string]*SiteInfo
	mu               sync.RWMutex
	options          CheckOptions
	client           *http.Client
	onUpdateCallback func([]*SiteInfo) // Callback for notifying updates
}

// NewSiteChecker creates a new site checker
func NewSiteChecker(options CheckOptions) *SiteChecker {
	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: settings.HTTPSettings.InsecureSkipVerify,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   options.Timeout,
	}

	if !options.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else if options.MaxRedirects > 0 {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= options.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", options.MaxRedirects)
			}
			return nil
		}
	}

	return &SiteChecker{
		sites:   make(map[string]*SiteInfo),
		options: options,
		client:  client,
	}
}

// SetUpdateCallback sets the callback function for site updates
func (sc *SiteChecker) SetUpdateCallback(callback func([]*SiteInfo)) {
	sc.onUpdateCallback = callback
}

// CollectSites collects URLs from enabled indexed sites only
func (sc *SiteChecker) CollectSites() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Clear existing sites
	sc.sites = make(map[string]*SiteInfo)

	// Debug: log indexed sites count
	logger.Debugf("Found %d indexed sites", len(site.IndexedSites))

	// Collect URLs from indexed sites, but only from enabled sites
	for siteName, indexedSite := range site.IndexedSites {
		// Check site status - only collect from enabled sites
		siteStatus := site.GetSiteStatus(siteName)
		if siteStatus != site.StatusEnabled {
			logger.Debugf("Skipping site %s (status: %s) - only collecting from enabled sites", siteName, siteStatus)
			continue
		}

		logger.Debugf("Processing enabled site: %s with %d URLs", siteName, len(indexedSite.Urls))
		for _, url := range indexedSite.Urls {
			if url != "" {
				logger.Debugf("Adding site URL: %s", url)
				// Load site config to determine display URL
				config, err := LoadSiteConfig(url)
				protocol := "http" // default protocol
				if err == nil && config != nil && config.HealthCheckConfig != nil && config.HealthCheckConfig.Protocol != "" {
					protocol = config.HealthCheckConfig.Protocol
					logger.Debugf("Site %s using protocol: %s", url, protocol)
				} else {
					logger.Debugf("Site %s using default protocol: %s (config error: %v)", url, protocol, err)
				}

				// Parse URL components for legacy fields

				// Get or create site config to get ID
				siteConfig := getOrCreateSiteConfigForURL(url)

				siteInfo := &SiteInfo{
					SiteConfig:  *siteConfig,
					Name:        extractDomainName(url),
					Status:      StatusChecking,
					LastChecked: time.Now().Unix(),
				}
				sc.sites[url] = siteInfo
			}
		}
	}

	logger.Debugf("Collected %d enabled sites for checking", len(sc.sites))
}

// loadAllSiteConfigs loads all site configs from database and caches them
func loadAllSiteConfigs() error {
	siteConfigMutex.Lock()
	defer siteConfigMutex.Unlock()

	// Skip database operation if query.SiteConfig is nil (e.g., in tests)
	if query.SiteConfig == nil {
		logger.Debugf("Skipping site config batch load - query.SiteConfig is nil (likely in test environment)")
		lastBatchLoad = time.Now()
		return nil
	}

	sc := query.SiteConfig
	configs, err := sc.Find()
	if err != nil {
		return fmt.Errorf("failed to load site configs: %w", err)
	}

	now := time.Now()
	expiry := now.Add(cacheExpiry)

	// Clear existing cache
	siteConfigCache = make(map[string]*siteConfigCacheEntry)

	// Cache all configs
	for _, config := range configs {
		siteConfigCache[config.Host] = &siteConfigCacheEntry{
			config:    config,
			expiresAt: expiry,
		}
	}

	lastBatchLoad = now
	logger.Debugf("Loaded %d site configs into cache", len(configs))
	return nil
}

// getCachedSiteConfig gets a site config from cache, loading all configs if needed
func getCachedSiteConfig(host string) (*model.SiteConfig, bool) {
	siteConfigMutex.RLock()

	// Check if we need to refresh the cache
	needsRefresh := time.Since(lastBatchLoad) > cacheExpiry

	if needsRefresh {
		siteConfigMutex.RUnlock()
		// Reload all configs if cache is expired
		if err := loadAllSiteConfigs(); err != nil {
			logger.Errorf("Failed to reload site configs: %v", err)
			return nil, false
		}
		siteConfigMutex.RLock()
	}

	entry, exists := siteConfigCache[host]
	siteConfigMutex.RUnlock()

	if !exists || time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.config, true
}

// setCachedSiteConfig sets a site config in cache
func setCachedSiteConfig(host string, config *model.SiteConfig) {
	siteConfigMutex.Lock()
	defer siteConfigMutex.Unlock()

	siteConfigCache[host] = &siteConfigCacheEntry{
		config:    config,
		expiresAt: time.Now().Add(cacheExpiry),
	}
}

// InvalidateSiteConfigCache invalidates the entire site config cache
func InvalidateSiteConfigCache() {
	siteConfigMutex.Lock()
	defer siteConfigMutex.Unlock()

	siteConfigCache = make(map[string]*siteConfigCacheEntry)
	lastBatchLoad = time.Time{} // Reset batch load time to force reload
	logger.Debugf("Site config cache invalidated")
}

// InvalidateSiteConfigCacheForHost invalidates cache for a specific host
func InvalidateSiteConfigCacheForHost(host string) {
	siteConfigMutex.Lock()
	defer siteConfigMutex.Unlock()

	delete(siteConfigCache, host)
	logger.Debugf("Site config cache invalidated for host: %s", host)
}

// getOrCreateSiteConfigForURL gets or creates a site config for the given URL
func getOrCreateSiteConfigForURL(url string) *model.SiteConfig {
	// Parse URL to get host:port
	tempConfig := &model.SiteConfig{}
	tempConfig.SetFromURL(url)

	// Try to get from cache first
	if config, found := getCachedSiteConfig(tempConfig.Host); found {
		return config
	}

	// Not in cache, query database
	sc := query.SiteConfig
	siteConfig, err := sc.Where(sc.Host.Eq(tempConfig.Host)).First()
	if err != nil {
		// Record doesn't exist, create a new one
		newConfig := &model.SiteConfig{
			Host:               tempConfig.Host,
			Port:               tempConfig.Port,
			Scheme:             tempConfig.Scheme,
			DisplayURL:         url,
			HealthCheckEnabled: true,
			CheckInterval:      300,
			Timeout:            10,
			UserAgent:          "Nginx-UI Site Checker/1.0",
			MaxRedirects:       3,
			FollowRedirects:    true,
			CheckFavicon:       true,
		}

		// Create the record in database
		if err := sc.Create(newConfig); err != nil {
			logger.Errorf("Failed to create site config for %s: %v", url, err)
			// Return temp config with a fake ID to avoid crashes
			tempConfig.ID = 0
			return tempConfig
		}

		// Cache the new config
		setCachedSiteConfig(tempConfig.Host, newConfig)
		return newConfig
	}

	// Record exists, ensure it has the correct URL information
	if siteConfig.DisplayURL == "" {
		siteConfig.DisplayURL = url
		siteConfig.SetFromURL(url)
		// Try to save the updated config, but don't fail if it doesn't work
		sc.Save(siteConfig)
	}

	// Cache the config
	setCachedSiteConfig(tempConfig.Host, siteConfig)
	return siteConfig
}

// CheckSite checks a single site's availability
func (sc *SiteChecker) CheckSite(ctx context.Context, siteURL string) (*SiteInfo, error) {
	// Try enhanced health check first if config exists
	config, err := LoadSiteConfig(siteURL)

	// If health check is disabled, return cached metadata without issuing any network requests (#1446)
	if err == nil && config != nil && !config.HealthCheckEnabled {
		siteInfo := &SiteInfo{
			SiteConfig: *config,
			Name:       extractDomainName(siteURL),
			Title:      config.DisplayURL,
		}

		if existing := sc.getExistingSiteSnapshot(siteURL); existing != nil {
			siteInfo.FaviconURL = existing.FaviconURL
			siteInfo.FaviconData = existing.FaviconData
			siteInfo.Status = existing.Status
			siteInfo.StatusCode = existing.StatusCode
			siteInfo.ResponseTime = existing.ResponseTime
			siteInfo.LastChecked = existing.LastChecked
			siteInfo.Error = existing.Error
			if siteInfo.Title == "" {
				siteInfo.Title = existing.Title
			}
		}

		return siteInfo, nil
	}

	if err == nil && config != nil && config.HealthCheckConfig != nil {
		enhancedChecker := NewEnhancedSiteChecker()
		siteInfo, err := enhancedChecker.CheckSiteWithConfig(ctx, siteURL, config.HealthCheckConfig)
		if err == nil && siteInfo != nil {
			// Fill in additional details
			siteInfo.ID = config.ID
			siteInfo.HealthCheckEnabled = config.HealthCheckEnabled
			siteInfo.Name = extractDomainName(siteURL)
			siteInfo.LastChecked = time.Now().Unix()

			// Set health check protocol and display URL
			siteInfo.DisplayURL = generateDisplayURL(siteURL, config.HealthCheckConfig.Protocol)

			// Try to get favicon if enabled and not a gRPC check
			if sc.options.CheckFavicon && !isGRPCProtocol(config.HealthCheckConfig.Protocol) {
				faviconURL, faviconData := sc.tryGetFavicon(ctx, siteURL)
				siteInfo.FaviconURL = faviconURL
				siteInfo.FaviconData = faviconData
			}

			return siteInfo, nil
		}
	}

	// Fallback to basic HTTP check, but preserve original protocol if available
	originalProtocol := "http" // default
	if config != nil && config.HealthCheckConfig != nil && config.HealthCheckConfig.Protocol != "" {
		originalProtocol = config.HealthCheckConfig.Protocol
	}
	return sc.checkSiteBasic(ctx, siteURL, originalProtocol)
}

// checkSiteBasic performs basic HTTP health check
func (sc *SiteChecker) checkSiteBasic(ctx context.Context, siteURL string, originalProtocol string) (*SiteInfo, error) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, "GET", siteURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", sc.options.UserAgent)

	resp, err := sc.client.Do(req)
	if err != nil {
		// Get or create site config to get ID
		siteConfig := getOrCreateSiteConfigForURL(siteURL)

		return &SiteInfo{
			SiteConfig:   *siteConfig,
			Name:         extractDomainName(siteURL),
			Status:       StatusOffline,
			ResponseTime: time.Since(start).Milliseconds(),
			LastChecked:  time.Now().Unix(),
			Error:        err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	responseTime := time.Since(start).Milliseconds()

	// Get or create site config to get ID
	siteConfig := getOrCreateSiteConfigForURL(siteURL)

	siteInfo := &SiteInfo{
		SiteConfig:   *siteConfig,
		Name:         extractDomainName(siteURL),
		StatusCode:   resp.StatusCode,
		ResponseTime: responseTime,
		LastChecked:  time.Now().Unix(),
	}

	// Determine status based on status code
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		siteInfo.Status = StatusOnline
	} else {
		siteInfo.Status = StatusError
		siteInfo.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	// Read response body for title and favicon extraction
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warnf("Failed to read response body for %s: %v", siteURL, err)
		return siteInfo, nil
	}

	// Extract title
	siteInfo.Title = extractTitle(string(body))

	// Extract favicon if enabled
	if sc.options.CheckFavicon {
		faviconURL, faviconData := sc.extractFavicon(ctx, siteURL, string(body))
		siteInfo.FaviconURL = faviconURL
		siteInfo.FaviconData = faviconData
	}

	return siteInfo, nil
}

// tryGetFavicon attempts to get favicon for enhanced checks
func (sc *SiteChecker) tryGetFavicon(ctx context.Context, siteURL string) (string, string) {
	// Make a simple GET request to get the HTML
	req, err := http.NewRequestWithContext(ctx, "GET", siteURL, nil)
	if err != nil {
		return "", ""
	}

	req.Header.Set("User-Agent", sc.options.UserAgent)

	resp, err := sc.client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", ""
	}

	return sc.extractFavicon(ctx, siteURL, string(body))
}

// CheckAllSites checks all collected sites concurrently
func (sc *SiteChecker) CheckAllSites(ctx context.Context) {
	sc.mu.RLock()
	urls := make([]string, 0, len(sc.sites))
	for url := range sc.sites {
		urls = append(urls, url)
	}
	sc.mu.RUnlock()

	// Use a semaphore to limit concurrent requests
	semaphore := make(chan struct{}, 10) // Max 10 concurrent requests
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		go func(siteURL string) {
			defer wg.Done()

			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			siteInfo, err := sc.CheckSite(ctx, siteURL)
			if err != nil {
				logger.Errorf("Failed to check site %s: %v", siteURL, err)
				return
			}

			sc.mu.Lock()
			sc.sites[siteURL] = siteInfo
			sc.mu.Unlock()
		}(url)
	}

	wg.Wait()
	// logger.Infof("Completed checking %d sites", len(urls))

	// Notify WebSocket clients of the update
	if sc.onUpdateCallback != nil {
		sites := make([]*SiteInfo, 0, len(sc.sites))
		sc.mu.RLock()
		for _, site := range sc.sites {
			sites = append(sites, site)
		}
		sc.mu.RUnlock()
		sc.onUpdateCallback(sites)
	}
}

// GetSites returns all checked sites
func (sc *SiteChecker) GetSites() map[string]*SiteInfo {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[string]*SiteInfo)
	maps.Copy(result, sc.sites)
	return result
}

// GetSiteCount returns the number of sites being monitored
func (sc *SiteChecker) GetSiteCount() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return len(sc.sites)
}

// GetSitesList returns sites as a slice
func (sc *SiteChecker) GetSitesList() []*SiteInfo {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	result := make([]*SiteInfo, 0, len(sc.sites))
	for _, site := range sc.sites {
		result = append(result, site)
	}
	return result
}

// getExistingSiteSnapshot returns a copy of the last known site info, if present.
func (sc *SiteChecker) getExistingSiteSnapshot(siteURL string) *SiteInfo {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	existing, ok := sc.sites[siteURL]
	if !ok || existing == nil {
		return nil
	}

	clone := *existing
	return &clone
}

// extractDomainName extracts domain name from URL
func extractDomainName(siteURL string) string {
	parsed, err := url.Parse(siteURL)
	if err != nil {
		return siteURL
	}
	return parsed.Host
}

// extractTitle extracts title from HTML content
func extractTitle(html string) string {
	titleRegex := regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	matches := titleRegex.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// extractFavicon extracts favicon URL and data from HTML
func (sc *SiteChecker) extractFavicon(ctx context.Context, siteURL, html string) (string, string) {
	parsedURL, err := url.Parse(siteURL)
	if err != nil {
		return "", ""
	}

	// Look for favicon link in HTML
	faviconRegex := regexp.MustCompile(`(?i)<link[^>]*rel=["'](?:icon|shortcut icon)["'][^>]*href=["']([^"']+)["']`)
	matches := faviconRegex.FindStringSubmatch(html)

	var faviconURL string
	if len(matches) > 1 {
		faviconURL = matches[1]
	} else {
		// Default favicon location
		faviconURL = "/favicon.ico"
	}

	// Convert relative URL to absolute
	if !strings.HasPrefix(faviconURL, "http") {
		baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
		if strings.HasPrefix(faviconURL, "/") {
			faviconURL = baseURL + faviconURL
		} else {
			faviconURL = baseURL + "/" + faviconURL
		}
	}

	// Download favicon
	faviconData := sc.downloadFavicon(ctx, faviconURL)

	return faviconURL, faviconData
}

// downloadFavicon downloads and encodes favicon as base64
func (sc *SiteChecker) downloadFavicon(ctx context.Context, faviconURL string) string {
	req, err := http.NewRequestWithContext(ctx, "GET", faviconURL, nil)
	if err != nil {
		return ""
	}

	req.Header.Set("User-Agent", sc.options.UserAgent)

	resp, err := sc.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	// Limit favicon size to 1MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return ""
	}

	headerContentType := normalizeContentType(resp.Header.Get("Content-Type"))
	inferredContentType := inferContentTypeFromURL(faviconURL)
	sniffedContentType := normalizeContentType(http.DetectContentType(body))

	contentType := headerContentType
	if !isAllowedFaviconContentType(contentType) && isAllowedFaviconContentType(sniffedContentType) {
		contentType = sniffedContentType
	}
	if !isAllowedFaviconContentType(contentType) &&
		headerContentType == "" &&
		isUnknownContentType(sniffedContentType) &&
		isAllowedFaviconContentType(inferredContentType) {
		contentType = inferredContentType
	}
	if !isAllowedFaviconContentType(contentType) {
		return ""
	}

	encoded := base64.StdEncoding.EncodeToString(body)
	return fmt.Sprintf("data:%s;base64,%s", contentType, encoded)
}

func normalizeContentType(contentType string) string {
	if contentType == "" {
		return ""
	}
	if semi := strings.Index(contentType, ";"); semi != -1 {
		contentType = contentType[:semi]
	}
	return strings.TrimSpace(strings.ToLower(contentType))
}

func inferContentTypeFromURL(faviconURL string) string {
	lower := strings.ToLower(faviconURL)
	switch {
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(lower, ".ico"):
		return "image/x-icon"
	default:
		return ""
	}
}

func isAllowedFaviconContentType(contentType string) bool {
	switch contentType {
	case "image/png",
		"image/jpeg",
		"image/webp",
		"image/gif",
		"image/svg+xml",
		"image/x-icon",
		"image/vnd.microsoft.icon":
		return true
	default:
		return false
	}
}

func isUnknownContentType(contentType string) bool {
	return contentType == "" || contentType == "application/octet-stream"
}

// generateDisplayURL generates the URL to display in UI based on health check protocol
func generateDisplayURL(originalURL, protocol string) string {
	parsed, err := url.Parse(originalURL)
	if err != nil {
		logger.Errorf("Failed to parse URL %s: %v", originalURL, err)
		return originalURL
	}

	// Determine the optimal scheme (prefer HTTPS if available)
	scheme := determineOptimalScheme(parsed, protocol)
	hostname := parsed.Hostname()
	port := parsed.Port()

	// For HTTP/HTTPS, return clean URL without default ports
	if scheme == "http" || scheme == "https" {
		// Build URL without default ports
		var result string
		if port == "" || (port == "80" && scheme == "http") || (port == "443" && scheme == "https") {
			// No port or default port - don't show port
			result = fmt.Sprintf("%s://%s", scheme, hostname)
		} else {
			// Non-default port - show port
			result = fmt.Sprintf("%s://%s:%s", scheme, hostname, port)
		}
		return result
	}

	// For gRPC/gRPCS, show the connection address format without default ports
	if scheme == "grpc" || scheme == "grpcs" {
		if port == "" {
			// Determine default port based on scheme
			if scheme == "grpcs" {
				port = "443"
			} else {
				port = "80"
			}
		}

		// Don't show default ports for gRPC either
		var result string
		if (port == "80" && scheme == "grpc") || (port == "443" && scheme == "grpcs") {
			result = fmt.Sprintf("%s://%s", scheme, hostname)
		} else {
			result = fmt.Sprintf("%s://%s:%s", scheme, hostname, port)
		}
		return result
	}

	// Fallback to original URL
	return originalURL
}

// isGRPCProtocol checks if the protocol is gRPC-based
func isGRPCProtocol(protocol string) bool {
	return protocol == "grpc" || protocol == "grpcs"
}

// parseURLComponents extracts scheme and host:port from URL based on health check protocol
func parseURLComponents(originalURL, healthCheckProtocol string) (scheme, hostPort string) {
	parsed, err := url.Parse(originalURL)
	if err != nil {
		logger.Debugf("Failed to parse URL %s: %v", originalURL, err)
		return healthCheckProtocol, originalURL
	}

	// Determine the best scheme to use
	scheme = determineOptimalScheme(parsed, healthCheckProtocol)

	// Extract hostname and port
	hostname := parsed.Hostname()
	if hostname == "" {
		// Fallback to original URL if we can't parse hostname
		return scheme, originalURL
	}

	port := parsed.Port()
	if port == "" {
		// Use default port based on scheme, but don't include it in hostPort for default ports
		switch scheme {
		case "https", "grpcs":
			// Default HTTPS port 443 - don't show in hostPort
			hostPort = hostname
		case "http", "grpc":
			// Default HTTP port 80 - don't show in hostPort
			hostPort = hostname
		default:
			hostPort = hostname
		}
	} else {
		// Non-default port specified
		isDefaultPort := (port == "80" && (scheme == "http" || scheme == "grpc")) ||
			(port == "443" && (scheme == "https" || scheme == "grpcs"))

		if isDefaultPort {
			// Don't show default ports
			hostPort = hostname
		} else {
			// Show non-default ports
			hostPort = hostname + ":" + port
		}
	}

	return scheme, hostPort
}

// determineOptimalScheme determines the best scheme to use based on original URL and health check protocol
func determineOptimalScheme(parsed *url.URL, healthCheckProtocol string) string {
	// If health check protocol is specified, use it, but with special handling for HTTP/HTTPS
	if healthCheckProtocol != "" {
		// Special case: Don't downgrade HTTPS to HTTP
		if healthCheckProtocol == "http" && parsed.Scheme == "https" {
			// logger.Debugf("Preserving HTTPS scheme instead of downgrading to HTTP")
			return "https"
		}

		// For gRPC protocols, always use the specified protocol
		if healthCheckProtocol == "grpc" || healthCheckProtocol == "grpcs" {
			return healthCheckProtocol
		}

		// For HTTPS health check protocol, always use HTTPS
		if healthCheckProtocol == "https" {
			return "https"
		}

		// For HTTP health check protocol, only use HTTP if original was also HTTP
		if healthCheckProtocol == "http" && parsed.Scheme == "http" {
			return "http"
		}
	}

	// If no health check protocol, or if we need to fall back, prefer HTTPS if the original URL is HTTPS
	if parsed.Scheme == "https" {
		return "https"
	}

	// Default to HTTP
	return "http"
}
