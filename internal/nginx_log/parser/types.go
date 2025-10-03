package parser

import (
	"regexp"
	"time"
)

// AccessLogEntry represents a parsed access log entry
type AccessLogEntry struct {
	ID           string   `json:"id"`
	Timestamp    int64    `json:"timestamp"` // Unix timestamp
	IP           string   `json:"ip"`
	RegionCode   string   `json:"region_code"`
	Province     string   `json:"province"`
	City         string   `json:"city"`
	Method       string   `json:"method"`
	Path         string   `json:"path"`
	Protocol     string   `json:"protocol"`
	Status       int      `json:"status"`
	BytesSent    int64    `json:"bytes_sent"`
	Referer      string   `json:"referer"`
	UserAgent    string   `json:"user_agent"`
	Browser      string   `json:"browser"`
	BrowserVer   string   `json:"browser_version"`
	OS           string   `json:"os"`
	OSVersion    string   `json:"os_version"`
	DeviceType   string   `json:"device_type"`
	RequestTime  float64  `json:"request_time"`
	UpstreamTime *float64 `json:"upstream_time,omitempty"`
	Raw          string   `json:"raw"`
}

// LogFormat represents different nginx log format patterns
type LogFormat struct {
	Name    string
	Pattern *regexp.Regexp
	Fields  []string
}

// UserAgentParser interface for user agent parsing
type UserAgentParser interface {
	Parse(userAgent string) UserAgentInfo
}

// UserAgentInfo represents parsed user agent information
type UserAgentInfo struct {
	Browser    string
	BrowserVer string
	OS         string
	OSVersion  string
	DeviceType string
}

// GeoIPService interface for geographic IP lookup
type GeoIPService interface {
	Search(ip string) (*GeoLocation, error)
}

// GeoLocation represents geographic location data
type GeoLocation struct {
	CountryCode string
	RegionCode  string
	Province    string
	City        string
}

// ParseResult represents the result of parsing operation
type ParseResult struct {
	Entries   []*AccessLogEntry
	Processed int
	Succeeded int
	Failed    int
	Duration  time.Duration
	ErrorRate float64
}

// ParserConfig holds configuration for the log parser
type Config struct {
	BufferSize    int
	BatchSize     int
	WorkerCount   int
	EnableGeoIP   bool
	EnableUA      bool
	TimeLayout    string
	StrictMode    bool
	MaxLineLength int
}

// DefaultParserConfig returns default parser configuration
func DefaultParserConfig() *Config {
	return &Config{
		BufferSize:    64 * 1024, // 64KB
		BatchSize:     1000,
		WorkerCount:   4,
		EnableGeoIP:   true,
		EnableUA:      true,
		TimeLayout:    "02/Jan/2006:15:04:05 -0700",
		StrictMode:    false,
		MaxLineLength: 16 * 1024, // 16KB max line length
	}
}

// ValidHTTPMethods Valid HTTP methods
var ValidHTTPMethods = map[string]bool{
	"GET":     true,
	"POST":    true,
	"PUT":     true,
	"DELETE":  true,
	"HEAD":    true,
	"OPTIONS": true,
	"PATCH":   true,
	"TRACE":   true,
	"CONNECT": true,
}

// Parser errors (moved to errors.go as Cosy Errors)
const (
	ErrInvalidStatus = "invalid status code"
)
