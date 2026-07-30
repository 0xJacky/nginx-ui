package model

import (
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
func (sc *SiteConfig) SetFromURL(url string) error {
	// Parse URL to extract host, port, and scheme
	// This is a simplified implementation - you may want to use net/url package
	if url == "" {
		return nil
	}

	// Store the original URL as display URL for backward compatibility
	sc.DisplayURL = url

	// Extract scheme
	if strings.HasPrefix(url, "https://") {
		sc.Scheme = "https"
		url = strings.TrimPrefix(url, "https://")
	} else if strings.HasPrefix(url, "http://") {
		sc.Scheme = "http"
		url = strings.TrimPrefix(url, "http://")
	} else if strings.HasPrefix(url, "grpcs://") {
		sc.Scheme = "grpcs"
		url = strings.TrimPrefix(url, "grpcs://")
	} else if strings.HasPrefix(url, "grpc://") {
		sc.Scheme = "grpc"
		url = strings.TrimPrefix(url, "grpc://")
	} else {
		sc.Scheme = "http" // default
	}

	// Extract host and port
	if strings.Contains(url, "/") {
		url = strings.Split(url, "/")[0]
	}

	if strings.Contains(url, ":") {
		parts := strings.Split(url, ":")
		sc.Host = parts[0] + ":" + parts[1]
		if len(parts) > 1 {
			if port, err := strconv.Atoi(parts[1]); err == nil {
				sc.Port = port
			}
		}
	} else {
		sc.Host = url + ":80" // default port
		sc.Port = 80
		if sc.Scheme == "https" || sc.Scheme == "grpcs" {
			sc.Host = url + ":443"
			sc.Port = 443
		}
	}

	return nil
}
