package nginx_log

import (
	"regexp"
	"time"
)

// AccessLogEntry represents a parsed access log entry
type AccessLogEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	IP           string    `json:"ip"`
	RegionCode   string    `json:"region_code"`
	Province     string    `json:"province"`
	City         string    `json:"city"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	Protocol     string    `json:"protocol"`
	Status       int       `json:"status"`
	BytesSent    int64     `json:"bytes_sent"`
	Referer      string    `json:"referer"`
	UserAgent    string    `json:"user_agent"`
	Browser      string    `json:"browser"`
	BrowserVer   string    `json:"browser_version"`
	OS           string    `json:"os"`
	OSVersion    string    `json:"os_version"`
	DeviceType   string    `json:"device_type"`
	RequestTime  float64   `json:"request_time,omitempty"`
	UpstreamTime *float64  `json:"upstream_time,omitempty"`
	Raw          string    `json:"raw"`
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

// Constants for optimization
const (
	invalidIPString = "invalid"
)

// Valid HTTP methods according to RFC specifications
var validHTTPMethods = map[string]bool{
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

// Common nginx log formats
var (
	// Standard combined log format
	CombinedFormat = &LogFormat{
		Name:    "combined",
		Pattern: regexp.MustCompile(`^(\S+) - (\S+) \[([^]]+)\] "([^"]*)" (\d+) (\d+|-) "([^"]*)" "([^"]*)"(?:\s+(\S+))?(?:\s+(\S+))?`),
		Fields:  []string{"ip", "remote_user", "timestamp", "request", "status", "bytes_sent", "referer", "user_agent", "request_time", "upstream_time"},
	}

	// Standard main log format
	MainFormat = &LogFormat{
		Name:    "main",
		Pattern: regexp.MustCompile(`^(\S+) - (\S+) \[([^]]+)\] "([^"]*)" (\d+) (\d+|-) "([^"]*)" "([^"]*)"`),
		Fields:  []string{"ip", "remote_user", "timestamp", "request", "status", "bytes_sent", "referer", "user_agent"},
	}

	// Custom format with more details
	DetailedFormat = &LogFormat{
		Name:    "detailed",
		Pattern: regexp.MustCompile(`^(\S+) - (\S+) \[([^]]+)\] "([^"]*)" (\d+) (\d+|-) "([^"]*)" "([^"]*)" (\S+) (\S+) "([^"]*)" (\S+)`),
		Fields:  []string{"ip", "remote_user", "timestamp", "request", "status", "bytes_sent", "referer", "user_agent", "request_time", "upstream_time", "x_forwarded_for", "connection"},
	}

	// All supported formats
	SupportedFormats = []*LogFormat{DetailedFormat, CombinedFormat, MainFormat}
)

// DetectLogFormat tries to detect the log format from sample lines
func DetectLogFormat(lines []string) *LogFormat {
	if len(lines) == 0 {
		return nil
	}

	for _, format := range SupportedFormats {
		matchCount := 0
		for _, line := range lines {
			if format.Pattern.MatchString(line) {
				matchCount++
			}
		}
		// If more than 50% of lines match, consider it a match
		if float64(matchCount)/float64(len(lines)) > 0.5 {
			return format
		}
	}

	return nil
}