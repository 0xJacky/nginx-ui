package sitecheck

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const enhancedClientTimeout = 30 * time.Second

// CheckResult bundles the SiteInfo with the response body so callers can
// reuse the body (e.g. for favicon extraction) without issuing a second
// request to the same host.
type CheckResult struct {
	Info *SiteInfo
	Body []byte
}

// EnhancedSiteChecker provides advanced health checking capabilities. It
// reuses the package-level shared HTTP transport for connection pooling.
type EnhancedSiteChecker struct {
	defaultClient *http.Client
}

// NewEnhancedSiteChecker creates a new enhanced site checker that reuses the
// shared transport.
func NewEnhancedSiteChecker() *EnhancedSiteChecker {
	return &EnhancedSiteChecker{
		defaultClient: SharedClient(enhancedClientTimeout),
	}
}

// rewriteCheckURLScheme aligns the scheme of siteURL with the configured
// healthcheck protocol while preserving path, query, fragment, and port.
// An unparseable URL is returned unchanged.
func rewriteCheckURLScheme(siteURL, protocol string) string {
	parsed, err := url.Parse(siteURL)
	if err != nil {
		return siteURL
	}
	parsed.Scheme = determineOptimalScheme(parsed, protocol)
	return parsed.String()
}

// CheckSiteWithConfig performs an enhanced health check using a custom
// configuration. The body of the HTTP/HTTPS response (if any) is returned so
// callers can reuse it; gRPC checks return a nil body.
func (ec *EnhancedSiteChecker) CheckSiteWithConfig(ctx context.Context, siteURL string, config *model.HealthCheckConfig) (*CheckResult, error) {
	return ec.checkSiteWithConfig(ctx, siteURL, config, nil)
}

func (ec *EnhancedSiteChecker) CheckSiteWithSiteConfig(ctx context.Context, siteURL string, siteConfig *model.SiteConfig) (*CheckResult, error) {
	if siteConfig == nil {
		return ec.CheckSiteWithConfig(ctx, siteURL, nil)
	}
	return ec.checkSiteWithConfig(ctx, siteURL, siteConfig.HealthCheckConfig, siteConfig)
}

func effectiveHealthCheckURL(siteURL string, config *model.HealthCheckConfig) string {
	if config != nil && strings.TrimSpace(config.TargetURL) != "" {
		return strings.TrimSpace(config.TargetURL)
	}
	return siteURL
}

func (ec *EnhancedSiteChecker) checkSiteWithConfig(ctx context.Context, siteURL string, config *model.HealthCheckConfig, siteConfig *model.SiteConfig) (*CheckResult, error) {
	if config == nil {
		return ec.checkHTTP(ctx, siteURL, &model.HealthCheckConfig{
			Protocol:       "http",
			Method:         "GET",
			Path:           "/",
			ExpectedStatus: []int{200},
		}, siteConfig)
	}

	// Align the request URL scheme with the configured healthcheck protocol
	// so HTTPS/gRPC checks don't silently fall back to the indexed HTTP URL.
	// Only the scheme is rewritten; path, query, and port are preserved.
	checkURL := rewriteCheckURLScheme(effectiveHealthCheckURL(siteURL, config), config.Protocol)

	switch config.Protocol {
	case "grpc", "grpcs":
		info, err := ec.checkGRPC(ctx, checkURL, config)
		if info == nil {
			return nil, err
		}
		return &CheckResult{Info: info}, err
	case "https":
		return ec.checkHTTPS(ctx, checkURL, config, siteConfig)
	default: // http
		return ec.checkHTTP(ctx, checkURL, config, siteConfig)
	}
}

// checkHTTP performs an HTTP health check and returns the response body so
// callers can reuse it (e.g. to extract a favicon) without re-fetching.
func (ec *EnhancedSiteChecker) checkHTTP(ctx context.Context, siteURL string, config *model.HealthCheckConfig, siteConfig *model.SiteConfig) (*CheckResult, error) {
	startTime := time.Now()

	// Build request URL
	checkURL := siteURL
	if config.Path != "" && config.Path != "/" {
		checkURL = strings.TrimRight(siteURL, "/") + "/" + strings.TrimLeft(config.Path, "/")
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, config.Method, checkURL, nil)
	if err != nil {
		return &CheckResult{Info: &SiteInfo{
			Status:    StatusError,
			Error:     fmt.Sprintf("Failed to create request: %v", err),
			ErrorType: ErrorTypeRequest,
		}}, err
	}

	// Add custom headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Set User-Agent if not provided
	if req.Header.Get("User-Agent") == "" {
		userAgent := "Nginx-UI Enhanced Checker/2.0"
		if siteConfig != nil && strings.TrimSpace(siteConfig.UserAgent) != "" {
			userAgent = siteConfig.UserAgent
		}
		req.Header.Set("User-Agent", userAgent)
	}

	// Add request body for POST/PUT methods
	if config.Body != "" && (config.Method == "POST" || config.Method == "PUT") {
		req.Body = io.NopCloser(strings.NewReader(config.Body))
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	// Copy the lightweight client value for every request so per-site timeout
	// and redirect policies never mutate a client shared by concurrent checks.
	// The underlying transport (and therefore its connection pool) remains
	// shared unless this site genuinely needs divergent TLS configuration.
	timeout := enhancedClientTimeout
	if siteConfig != nil && siteConfig.Timeout > 0 {
		timeout = time.Duration(siteConfig.Timeout) * time.Second
	}
	clientValue := *ec.defaultClient
	clientValue.Timeout = timeout
	client := &clientValue
	if needsCustomTLS(config) {
		client = ClientForHealthCheck(config, timeout)
	}
	if siteConfig != nil {
		if !siteConfig.FollowRedirects {
			client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			}
		} else if siteConfig.MaxRedirects > 0 {
			client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
				if len(via) >= siteConfig.MaxRedirects {
					return fmt.Errorf("stopped after %d redirects", siteConfig.MaxRedirects)
				}
				return nil
			}
		}
	}

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return &CheckResult{Info: &SiteInfo{
			Status:       StatusError,
			ResponseTime: time.Since(startTime).Milliseconds(),
			Error:        err.Error(),
			ErrorType:    classifyCheckError(err),
		}}, err
	}
	defer resp.Body.Close()

	responseTime := time.Since(startTime).Milliseconds()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warnf("Failed to read response body: %v", err)
		body = []byte{}
	}

	// Validate status code
	statusValid := false
	if len(config.ExpectedStatus) > 0 {
		statusValid = slices.Contains(config.ExpectedStatus, resp.StatusCode)
	} else {
		statusValid = resp.StatusCode >= 200 && resp.StatusCode < 400
	}

	// Validate response text
	bodyText := string(body)
	textValid := true
	if config.ExpectedText != "" {
		textValid = strings.Contains(bodyText, config.ExpectedText)
	}
	if config.NotExpectedText != "" {
		textValid = textValid && !strings.Contains(bodyText, config.NotExpectedText)
	}

	// Determine final status
	status := StatusOffline
	var errorMsg, errorType string
	if statusValid && textValid {
		status = StatusOnline
	} else {
		if !statusValid {
			errorMsg = fmt.Sprintf("Unexpected status code: %d", resp.StatusCode)
			errorType = ErrorTypeStatusCode
		} else {
			errorMsg = "Response content validation failed"
			errorType = ErrorTypeContent
		}
	}

	return &CheckResult{
		Info: &SiteInfo{
			Status:       status,
			StatusCode:   resp.StatusCode,
			ResponseTime: responseTime,
			Error:        errorMsg,
			ErrorType:    errorType,
		},
		Body: body,
	}, nil
}

// checkHTTPS performs HTTPS health check with SSL validation
func (ec *EnhancedSiteChecker) checkHTTPS(ctx context.Context, siteURL string, config *model.HealthCheckConfig, siteConfig *model.SiteConfig) (*CheckResult, error) {
	// Force HTTPS protocol
	httpsConfig := *config
	httpsConfig.Protocol = "https"
	return ec.checkHTTP(ctx, siteURL, &httpsConfig, siteConfig)
}

// checkGRPC performs gRPC health check
func (ec *EnhancedSiteChecker) checkGRPC(ctx context.Context, siteURL string, config *model.HealthCheckConfig) (*SiteInfo, error) {
	startTime := time.Now()

	// Parse URL to get host and port
	parsedURL, err := parseGRPCURL(siteURL)
	if err != nil {
		return &SiteInfo{
			Status:    StatusError,
			Error:     fmt.Sprintf("Invalid gRPC URL: %v", err),
			ErrorType: ErrorTypeRequest,
		}, err
	}

	// Set up connection options
	var opts []grpc.DialOption

	// TLS configuration based on protocol setting, not URL scheme
	if config.Protocol == "grpcs" || config.ValidateSSL {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(grpcTLSConfig(config))))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Create gRPC client (connection established lazily on first RPC call)
	conn, err := grpc.NewClient(parsedURL.Host, opts...)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to connect to gRPC server: %v", err)

		// Provide more specific error messages
		if strings.Contains(err.Error(), "connection refused") {
			errorMsg = fmt.Sprintf("Connection refused - server may not be running on %s", parsedURL.Host)
		} else if strings.Contains(err.Error(), "context deadline exceeded") {
			errorMsg = fmt.Sprintf("Connection timeout - server at %s did not respond within 5 seconds", parsedURL.Host)
		} else if strings.Contains(err.Error(), "EOF") {
			errorMsg = fmt.Sprintf("Protocol mismatch - %s may not be a gRPC server or wrong TLS configuration", parsedURL.Host)
		}

		return &SiteInfo{
			Status:       StatusError,
			ResponseTime: time.Since(startTime).Milliseconds(),
			Error:        errorMsg,
			ErrorType:    classifyCheckError(err),
		}, err
	}
	defer conn.Close()

	// Use health check service
	client := grpc_health_v1.NewHealthClient(conn)

	// Determine service name
	serviceName := ""
	if config.GRPCService != "" {
		serviceName = config.GRPCService
	}

	// Make health check request with shorter timeout
	checkCtx, checkCancel := context.WithTimeout(ctx, 3*time.Second)
	defer checkCancel()

	resp, err := client.Check(checkCtx, &grpc_health_v1.HealthCheckRequest{
		Service: serviceName,
	})

	responseTime := time.Since(startTime).Milliseconds()

	if err != nil {
		errorMsg := fmt.Sprintf("Health check failed: %v", err)

		// Provide more specific error messages for gRPC health check failures
		if strings.Contains(err.Error(), "Unimplemented") {
			errorMsg = "Server does not implement gRPC health check service"
		} else if strings.Contains(err.Error(), "context deadline exceeded") {
			errorMsg = "Health check timeout - server did not respond within 3 seconds"
		} else if strings.Contains(err.Error(), "EOF") {
			errorMsg = "Connection lost during health check"
		}

		return &SiteInfo{
			Status:       StatusError,
			ResponseTime: responseTime,
			Error:        errorMsg,
			ErrorType:    classifyCheckError(err),
		}, err
	}

	// Check response status
	status := StatusOffline
	if resp.Status == grpc_health_v1.HealthCheckResponse_SERVING {
		status = StatusOnline
	}

	return &SiteInfo{
		Status:       status,
		ResponseTime: responseTime,
	}, nil
}

// grpcTLSConfig builds the TLS configuration used by a gRPC health probe.
//
// It reuses probeVerifiesCertificate so the gRPC and HTTP paths cannot drift
// apart: the same site configuration produces the same trust decision on both.
// The previous implementation derived this from the global
// settings.HTTPSettings.InsecureSkipVerify, which defaults to false and
// therefore reported healthy internal grpcs endpoints as down (#1790).
func grpcTLSConfig(config *model.HealthCheckConfig) *tls.Config {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !probeVerifiesCertificate(config),
	}

	// Load client certificate if provided
	if config.ClientCert != "" && config.ClientKey != "" {
		cert, err := tls.LoadX509KeyPair(config.ClientCert, config.ClientKey)
		if err != nil {
			logger.Warnf("Failed to load client certificate: %v", err)
		} else {
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	return tlsConfig
}

// parseGRPCURL parses a URL and extracts host:port for gRPC connection
func parseGRPCURL(rawURL string) (*url.URL, error) {
	// Parse the original URL to extract host and port
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	// Create a new URL structure for gRPC connection
	grpcURL := &url.URL{
		Scheme: "grpc", // Default to grpc, will be overridden by config.Protocol
		Host:   parsedURL.Host,
	}

	// If no port is specified, use default ports based on original scheme
	if parsedURL.Port() == "" {
		switch parsedURL.Scheme {
		case "https":
			grpcURL.Host = parsedURL.Hostname() + ":443"
		case "http":
			grpcURL.Host = parsedURL.Hostname() + ":80"
		case "grpcs":
			grpcURL.Host = parsedURL.Hostname() + ":443"
		case "grpc":
			grpcURL.Host = parsedURL.Hostname() + ":80"
		default:
			// For URLs without scheme, default to port 80
			grpcURL.Host = parsedURL.Host + ":80"
		}
	}

	return grpcURL, nil
}

// LoadSiteConfig loads health check configuration for a site using cache
func LoadSiteConfig(siteName, siteURL string) (*model.SiteConfig, error) {
	// Parse URL to get host:port
	tempConfig := &model.SiteConfig{}
	tempConfig.SetFromURL(siteURL)
	tempConfig.SiteKey = canonicalSiteKey(siteName, siteURL)

	// Try to get from cache first
	if config, found := getCachedSiteConfig(tempConfig.SiteKey); found {
		// Set default health check config if nil
		if config.HealthCheckConfig == nil {
			config.HealthCheckConfig = &model.HealthCheckConfig{
				Protocol:       "http",
				Method:         "GET",
				Path:           "/",
				ExpectedStatus: []int{200},
			}
		}
		return config, nil
	}

	// Not in cache, query database
	sc := query.SiteConfig
	config, err := sc.Where(sc.SiteKey.Eq(tempConfig.SiteKey)).First()
	if err != nil && siteName == "" {
		config, err = sc.Where(sc.Host.Eq(tempConfig.Host)).First()
	}
	if err != nil {
		// Return default config if not found
		defaultConfig := &model.SiteConfig{
			HealthCheckEnabled: true,
			CheckInterval:      300,
			Timeout:            10,
			HealthCheckConfig: &model.HealthCheckConfig{
				Protocol:       "http",
				Method:         "GET",
				Path:           "/",
				ExpectedStatus: []int{200},
			},
		}
		defaultConfig.SetFromURL(siteURL)
		return defaultConfig, nil
	}

	// Set default health check config if nil
	if config.HealthCheckConfig == nil {
		config.HealthCheckConfig = &model.HealthCheckConfig{
			Protocol:       "http",
			Method:         "GET",
			Path:           "/",
			ExpectedStatus: []int{200},
		}
	}

	// Cache the config
	setCachedSiteConfig(tempConfig.SiteKey, config)
	return config, nil
}
