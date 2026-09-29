package parser

// AccessLogEntry represents a parsed access log entry
type AccessLogEntry struct {
	Timestamp    int64    `json:"timestamp"` // Unix timestamp
	IP           string   `json:"ip"`
	Method       string   `json:"method"`
	Path         string   `json:"path"`
	Protocol     string   `json:"protocol"`
	Status       int      `json:"status"`
	BytesSent    int64    `json:"bytes_sent"`
	Referer      string   `json:"referer"`
	UserAgent    string   `json:"user_agent"`
	RequestTime  float64  `json:"request_time"`
	UpstreamTime *float64 `json:"upstream_time,omitempty"`
	Raw          string   `json:"raw"`
}

// Config holds configuration for the log parser
type Config struct {
	TimeLayout    string
	StrictMode    bool
	MaxLineLength int
}

// DefaultParserConfig returns default parser configuration
func DefaultParserConfig() *Config {
	return &Config{
		TimeLayout:    "02/Jan/2006:15:04:05 -0700",
		StrictMode:    false,
		MaxLineLength: 16 * 1024, // 16KB max line length
	}
}

// ValidHTTPMethods Valid HTTP methods including WebDAV methods
var ValidHTTPMethods = map[string]bool{
	// Standard HTTP methods
	"GET":     true,
	"POST":    true,
	"PUT":     true,
	"DELETE":  true,
	"HEAD":    true,
	"OPTIONS": true,
	"PATCH":   true,
	"TRACE":   true,
	"CONNECT": true,
	// WebDAV methods (RFC 4918)
	"PROPFIND":  true,
	"PROPPATCH": true,
	"MKCOL":     true,
	"COPY":      true,
	"MOVE":      true,
	"LOCK":      true,
	"UNLOCK":    true,
}

// Parser errors (moved to errors.go as Cosy Errors)
const (
	ErrInvalidStatus = "invalid status code"
)
