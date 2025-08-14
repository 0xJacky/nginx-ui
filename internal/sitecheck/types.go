package sitecheck

import (
	"time"
)

// Site health check status constants
const (
	StatusOnline   = "online"
	StatusOffline  = "offline"
	StatusError    = "error"
	StatusChecking = "checking"
)

// SiteInfo represents the information about a site
type SiteInfo struct {
	ID           uint64 `json:"id"`          // Site config ID for API operations
	Host         string `json:"host"`        // host:port format
	Port         int    `json:"port"`        // port number
	Scheme       string `json:"scheme"`      // http, https, grpc, grpcs
	DisplayURL   string `json:"display_url"` // computed URL for display
	Name         string `json:"name"`
	Status       string `json:"status"` // StatusOnline, StatusOffline, StatusError, StatusChecking
	StatusCode   int    `json:"status_code"`
	ResponseTime int64  `json:"response_time"` // in milliseconds
	FaviconURL   string `json:"favicon_url"`
	FaviconData  string `json:"favicon_data"` // base64 encoded favicon
	Title        string `json:"title"`
	LastChecked  int64  `json:"last_checked"` // Unix timestamp in seconds
	Error        string `json:"error,omitempty"`
	// Legacy fields for backward compatibility
	URL                 string `json:"url,omitempty"`                   // deprecated, use display_url instead
	HealthCheckProtocol string `json:"health_check_protocol,omitempty"` // deprecated, use scheme instead
	HostPort            string `json:"host_port,omitempty"`             // deprecated, use host instead
}

// CheckOptions represents options for site checking
type CheckOptions struct {
	Timeout         time.Duration
	UserAgent       string
	FollowRedirects bool
	MaxRedirects    int
	CheckFavicon    bool
}

// DefaultCheckOptions returns default checking options
func DefaultCheckOptions() CheckOptions {
	return CheckOptions{
		Timeout:         10 * time.Second,
		UserAgent:       "Nginx-UI Site Checker/1.0",
		FollowRedirects: true,
		MaxRedirects:    3,
		CheckFavicon:    true,
	}
}
