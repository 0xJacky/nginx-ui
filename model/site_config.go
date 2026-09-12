package model

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type HealthCheckConfig struct {
	// Optional probe target. When empty, the URL discovered from the Nginx
	// server block is used.
	TargetURL string `json:"target_url"`

	// Protocol settings
	Protocol string            `json:"protocol"`                       // http, https, grpc
	Method   string            `json:"method"`                         // GET, POST, PUT, etc.
	Path     string            `json:"path"`                           // URL path to check
	Headers  map[string]string `json:"headers" gorm:"serializer:json"` // Custom headers
	Body     string            `json:"body"`                           // Request body for POST/PUT

	// Response validation
	ExpectedStatus  []int  `json:"expected_status" gorm:"serializer:json"` // Expected HTTP status codes
	ExpectedText    string `json:"expected_text"`                          // Text that should be present in response
	NotExpectedText string `json:"not_expected_text"`                      // Text that should NOT be present
	ValidateSSL     bool   `json:"validate_ssl"`                           // Validate SSL certificate

	// GRPC specific settings
	GRPCService string `json:"grpc_service"` // GRPC service name
	GRPCMethod  string `json:"grpc_method"`  // GRPC method name

	// Advanced settings
	DNSResolver    string `json:"dns_resolver"`    // Custom DNS resolver
	SourceIP       string `json:"source_ip"`       // Source IP for requests
	VerifyHostname bool   `json:"verify_hostname"` // Verify hostname in SSL cert
	ClientCert     string `json:"client_cert"`     // Client certificate path
	ClientKey      string `json:"client_key"`      // Client key path
}

type SiteHealthAlertConfig struct {
	Enabled           bool     `json:"enabled"`
	StatusCodes       []int    `json:"status_codes" gorm:"serializer:json"`
	NetworkErrors     bool     `json:"network_errors"`
	FailureThreshold  int      `json:"failure_threshold"`
	RecoveryEnabled   bool     `json:"recovery_enabled"`
	CooldownSeconds   int      `json:"cooldown_seconds"`
	ExternalNotifyIDs []uint64 `json:"external_notify_ids" gorm:"serializer:json"`
}

type SiteConfig struct {
	Model
	SiteKey            string                 `gorm:"index" json:"site_key" cosy:"all:omitempty"`
	SiteName           string                 `gorm:"index" json:"site_name" cosy:"all:omitempty"`
	Host               string                 `gorm:"index" json:"host" cosy:"all:omitempty"`            // host:port format
	Port               int                    `gorm:"index" json:"port" cosy:"all:omitempty"`            // port number
	Scheme             string                 `gorm:"default:'http'" json:"scheme" cosy:"all:omitempty"` // http, https, grpc, grpcs
	DisplayURL         string                 `json:"display_url" cosy:"all:omitempty"`                  // computed URL for display
	CustomOrder        int                    `gorm:"default:0" json:"custom_order" cosy:"all:omitempty"`
	HealthCheckEnabled bool                   `json:"health_check_enabled" cosy:"all:omitempty"`
	CheckInterval      int                    `gorm:"default:300" json:"check_interval" cosy:"all:omitempty"` // seconds
	Timeout            int                    `gorm:"default:10" json:"timeout" cosy:"all:omitempty"`         // seconds
	UserAgent          string                 `gorm:"default:'Nginx-UI Site Checker/1.0'" json:"user_agent" cosy:"all:omitempty"`
	MaxRedirects       int                    `gorm:"default:3" json:"max_redirects" cosy:"all:omitempty"`
	FollowRedirects    bool                   `gorm:"default:true" json:"follow_redirects" cosy:"all:omitempty"`
	CheckFavicon       bool                   `gorm:"default:true" json:"check_favicon" cosy:"all:omitempty"`
	HealthCheckConfig  *HealthCheckConfig     `gorm:"serializer:json" json:"health_check_config" cosy:"all:omitempty"`
	HealthCheckAlert   *SiteHealthAlertConfig `gorm:"serializer:json" json:"health_check_alert" cosy:"all:omitempty"`
}

// GetURL returns the computed URL for this site config
func (sc *SiteConfig) GetURL() string {
	if sc.DisplayURL != "" {
		return sc.DisplayURL
	}
	return sc.Scheme + "://" + sc.Host
}

// SetFromURL parses a URL and sets the Host, Port, and Scheme fields
func (sc *SiteConfig) SetFromURL(value string) error {
	rawURL := strings.TrimSpace(value)
	if rawURL == "" {
		return nil
	}

	// Store the original URL as display URL for backward compatibility
	sc.DisplayURL = value
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return fmt.Errorf("invalid site URL %q", value)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" && scheme != "grpc" && scheme != "grpcs" {
		return fmt.Errorf("unsupported site URL scheme %q", parsed.Scheme)
	}

	portText := parsed.Port()
	if portText == "" {
		if scheme == "https" || scheme == "grpcs" {
			portText = "443"
		} else {
			portText = "80"
		}
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid site URL port %q", portText)
	}

	sc.Scheme = scheme
	sc.Port = port
	sc.Host = net.JoinHostPort(parsed.Hostname(), portText)

	return nil
}
