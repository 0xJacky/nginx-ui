package sitecheck

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
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

// EnhancedSiteChecker provides advanced health checking capabilities
type EnhancedSiteChecker struct {
	defaultClient *http.Client
}

// NewEnhancedSiteChecker creates a new enhanced site checker
func NewEnhancedSiteChecker() *EnhancedSiteChecker {
	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return &EnhancedSiteChecker{
		defaultClient: client,
	}
}

// CheckSiteWithConfig performs enhanced health check using custom configuration
func (ec *EnhancedSiteChecker) CheckSiteWithConfig(ctx context.Context, siteURL string, config *model.HealthCheckConfig) (*SiteInfo, error) {
	if config == nil {
		// Fallback to basic HTTP check
		return ec.checkHTTP(ctx, siteURL, &model.HealthCheckConfig{
			Protocol:       "http",
			Method:         "GET",
			Path:           "/",
			ExpectedStatus: []int{200},
		})
	}

	switch config.Protocol {
	case "grpc", "grpcs":
		return ec.checkGRPC(ctx, siteURL, config)
	case "https":
		return ec.checkHTTPS(ctx, siteURL, config)
	default: // http
		return ec.checkHTTP(ctx, siteURL, config)
	}
}

// checkHTTP performs HTTP health check
func (ec *EnhancedSiteChecker) checkHTTP(ctx context.Context, siteURL string, config *model.HealthCheckConfig) (*SiteInfo, error) {
	startTime := time.Now()

	// Build request URL
	checkURL := siteURL
	if config.Path != "" && config.Path != "/" {
		checkURL = strings.TrimRight(siteURL, "/") + "/" + strings.TrimLeft(config.Path, "/")
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, config.Method, checkURL, nil)
	if err != nil {
		// Parse URL components for error case
		scheme, hostPort := parseURLComponents(siteURL, config.Protocol)

		return &SiteInfo{
			URL:                 siteURL,
			Status:              StatusError,
			Error:               fmt.Sprintf("Failed to create request: %v", err),
			HealthCheckProtocol: config.Protocol,
			Scheme:              scheme,
			HostPort:            hostPort,
		}, err
	}

	// Add custom headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Set User-Agent if not provided
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Nginx-UI Enhanced Checker/2.0")
	}

	// Add request body for POST/PUT methods
	if config.Body != "" && (config.Method == "POST" || config.Method == "PUT") {
		req.Body = io.NopCloser(strings.NewReader(config.Body))
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	// Create custom client if needed
	client := ec.defaultClient
	if config.ValidateSSL || config.VerifyHostname {
		transport := &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !config.ValidateSSL,
			},
		}

		// Load client certificate if provided
		if config.ClientCert != "" && config.ClientKey != "" {
			cert, err := tls.LoadX509KeyPair(config.ClientCert, config.ClientKey)
			if err != nil {
				logger.Warnf("Failed to load client certificate: %v", err)
			} else {
				transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
			}
		}

		client = &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}
	}

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		// Parse URL components for error case
		scheme, hostPort := parseURLComponents(siteURL, config.Protocol)

		return &SiteInfo{
			URL:                 siteURL,
			Status:              StatusError,
			ResponseTime:        time.Since(startTime).Milliseconds(),
			Error:               err.Error(),
			HealthCheckProtocol: config.Protocol,
			Scheme:              scheme,
			HostPort:            hostPort,
		}, err
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
	var errorMsg string
	if statusValid && textValid {
		status = StatusOnline
	} else {
		if !statusValid {
			errorMsg = fmt.Sprintf("Unexpected status code: %d", resp.StatusCode)
		} else {
			errorMsg = "Response content validation failed"
		}
	}

	// Parse URL components for legacy fields
	_, hostPort := parseURLComponents(siteURL, config.Protocol)

	// Get or create site config to get ID
	siteConfig := getOrCreateSiteConfigForURL(siteURL)

	return &SiteInfo{
		ID:           siteConfig.ID,
		Host:         siteConfig.Host,
		Port:         siteConfig.Port,
		Scheme:       siteConfig.Scheme,
		DisplayURL:   siteConfig.GetURL(),
		Status:       status,
		StatusCode:   resp.StatusCode,
		ResponseTime: responseTime,
		Error:        errorMsg,
		// Legacy fields for backward compatibility
		URL:                 siteURL,
		HealthCheckProtocol: config.Protocol,
		HostPort:            hostPort,
	}, nil
}

// checkHTTPS performs HTTPS health check with SSL validation
func (ec *EnhancedSiteChecker) checkHTTPS(ctx context.Context, siteURL string, config *model.HealthCheckConfig) (*SiteInfo, error) {
	// Force HTTPS protocol
	httpsConfig := *config
	httpsConfig.Protocol = "https"
	httpsConfig.ValidateSSL = true

	return ec.checkHTTP(ctx, siteURL, &httpsConfig)
}

// checkGRPC performs gRPC health check
func (ec *EnhancedSiteChecker) checkGRPC(ctx context.Context, siteURL string, config *model.HealthCheckConfig) (*SiteInfo, error) {
	startTime := time.Now()

	// Parse URL to get host and port
	parsedURL, err := parseGRPCURL(siteURL)
	if err != nil {
		// Parse URL components for error case
		scheme, hostPort := parseURLComponents(siteURL, config.Protocol)

		return &SiteInfo{
			URL:                 siteURL,
			Status:              StatusError,
			Error:               fmt.Sprintf("Invalid gRPC URL: %v", err),
			HealthCheckProtocol: config.Protocol,
			Scheme:              scheme,
			HostPort:            hostPort,
		}, err
	}

	// Set up connection options
	var opts []grpc.DialOption

	// TLS configuration based on protocol setting, not URL scheme
	if config.Protocol == "grpcs" || config.ValidateSSL {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: !config.ValidateSSL,
		}

		// For GRPCS, default to skip verification unless explicitly enabled
		if config.Protocol == "grpcs" && !config.ValidateSSL {
			tlsConfig.InsecureSkipVerify = true
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

		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Create connection with shorter timeout for faster failure detection
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, parsedURL.Host, opts...)
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

		// Parse URL components for error case
		scheme, hostPort := parseURLComponents(siteURL, config.Protocol)

		return &SiteInfo{
			URL:                 siteURL,
			Status:              StatusError,
			ResponseTime:        time.Since(startTime).Milliseconds(),
			Error:               errorMsg,
			HealthCheckProtocol: config.Protocol,
			Scheme:              scheme,
			HostPort:            hostPort,
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

		// Parse URL components for error case
		scheme, hostPort := parseURLComponents(siteURL, config.Protocol)

		return &SiteInfo{
			URL:                 siteURL,
			Status:              StatusError,
			ResponseTime:        responseTime,
			Error:               errorMsg,
			HealthCheckProtocol: config.Protocol,
			Scheme:              scheme,
			HostPort:            hostPort,
		}, err
	}

	// Check response status
	status := StatusOffline
	if resp.Status == grpc_health_v1.HealthCheckResponse_SERVING {
		status = StatusOnline
	}

	// Parse URL components
	scheme, hostPort := parseURLComponents(siteURL, config.Protocol)

	return &SiteInfo{
		URL:                 siteURL,
		Status:              status,
		ResponseTime:        responseTime,
		HealthCheckProtocol: config.Protocol,
		Scheme:              scheme,
		HostPort:            hostPort,
	}, nil
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

// LoadSiteConfig loads health check configuration for a site
func LoadSiteConfig(siteURL string) (*model.SiteConfig, error) {
	// Parse URL to get host:port
	tempConfig := &model.SiteConfig{}
	tempConfig.SetFromURL(siteURL)

	sc := query.SiteConfig
	config, err := sc.Where(sc.Host.Eq(tempConfig.Host)).First()
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

	return config, nil
}
