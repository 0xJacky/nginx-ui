package nginx_log

import (
	"net"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uozi-tech/cosy/geoip"
	"github.com/uozi-tech/cosy/logger"
)

// AccessLogEntry represents a parsed access log entry
type AccessLogEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	IP           string    `json:"ip"`
	Location     string    `json:"location"`
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

// LogParser handles parsing of nginx access logs
type LogParser struct {
	userAgent UserAgentParser
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

// NewLogParser creates a new log parser instance
func NewLogParser(userAgent UserAgentParser) *LogParser {
	return &LogParser{
		userAgent: userAgent,
	}
}

// ParseLine parses a single log line and returns a structured entry
func (p *LogParser) ParseLine(line string) (*AccessLogEntry, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, ErrEmptyLogLine
	}

	// Try to match against supported formats
	for _, format := range SupportedFormats {
		if matches := format.Pattern.FindStringSubmatch(line); matches != nil {
			return p.parseMatches(matches, format, line)
		}
	}

	// If no format matches, return raw entry
	return &AccessLogEntry{
		Raw: line,
	}, nil
}

// parseMatches converts regex matches to AccessLogEntry
func (p *LogParser) parseMatches(matches []string, format *LogFormat, rawLine string) (*AccessLogEntry, error) {
	entry := &AccessLogEntry{Raw: rawLine}

	for i, field := range format.Fields {
		if i+1 >= len(matches) {
			break
		}
		value := matches[i+1]

		switch field {
		case "ip":
			entry.IP = p.extractRealIP(value)
			entry.Location = geoip.ParseIP(entry.IP)

		case "timestamp":
			if timestamp, err := p.parseTimestamp(value); err == nil {
				entry.Timestamp = timestamp
			}

		case "request":
			entry.Method, entry.Path, entry.Protocol = p.parseRequest(value)

		case "status":
			if status, err := strconv.Atoi(value); err == nil {
				entry.Status = status
			}

		case "bytes_sent":
			if value != "-" {
				if bytes, err := strconv.ParseInt(value, 10, 64); err == nil {
					entry.BytesSent = bytes
				}
			}

		case "referer":
			if value != "-" {
				entry.Referer = value
			}

		case "user_agent":
			entry.UserAgent = value
			if p.userAgent != nil {
				uaInfo := p.userAgent.Parse(value)
				entry.Browser = uaInfo.Browser
				entry.BrowserVer = uaInfo.BrowserVer
				entry.OS = uaInfo.OS
				entry.OSVersion = uaInfo.OSVersion
				entry.DeviceType = uaInfo.DeviceType
			}

		case "request_time":
			if value != "-" {
				if reqTime, err := strconv.ParseFloat(value, 64); err == nil {
					entry.RequestTime = reqTime
				}
			}

		case "upstream_time":
			if value != "-" {
				if upTime, err := strconv.ParseFloat(value, 64); err == nil {
					entry.UpstreamTime = &upTime
				}
			}
		}
	}

	return entry, nil
}

// extractRealIP extracts the real client IP from various headers
func (p *LogParser) extractRealIP(ipStr string) string {
	// Handle X-Forwarded-For format: "client, proxy1, proxy2"
	if strings.Contains(ipStr, ",") {
		ips := strings.Split(ipStr, ",")
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			if p.isValidPublicIP(ip) {
				return ip
			}
		}
		// If no public IP found, return the first one
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	return ipStr
}

// isValidPublicIP checks if an IP is a valid public IP address
func (p *LogParser) isValidPublicIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// Check if it's a private IP
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsMulticast() {
		return false
	}

	return true
}

// parseTimestamp parses nginx timestamp format
func (p *LogParser) parseTimestamp(timestampStr string) (time.Time, error) {
	// Nginx default timestamp format: "02/Jan/2006:15:04:05 -0700"
	layouts := []string{
		"02/Jan/2006:15:04:05 -0700",
		"02/Jan/2006:15:04:05",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, timestampStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, ErrInvalidTimestamp
}

// parseRequest parses the HTTP request string
func (p *LogParser) parseRequest(requestStr string) (method, path, protocol string) {
	parts := strings.Fields(requestStr)
	if len(parts) >= 1 {
		method = parts[0]
	}
	if len(parts) >= 2 {
		path = parts[1]
	}
	if len(parts) >= 3 {
		protocol = parts[2]
	}
	return
}

// ParseLines parses multiple log lines and returns a slice of entries
func (p *LogParser) ParseLines(lines []string) []*AccessLogEntry {
	return p.ParseLinesParallel(lines)
}

// ParseLinesParallel parses multiple log lines using parallel workers with optimized ordering
func (p *LogParser) ParseLinesParallel(lines []string) []*AccessLogEntry {
	if len(lines) == 0 {
		return nil
	}

	// Calculate worker count: half of CPU cores, minimum 1
	numWorkers := runtime.NumCPU() / 2
	if numWorkers < 1 {
		numWorkers = 1
	}

	// For small datasets, use single-threaded parsing to avoid overhead
	if len(lines) < 100 || numWorkers == 1 {
		return p.parseLinesSingleThreaded(lines)
	}

	// Pre-allocate result array to maintain order without sorting - O(1) insertion
	results := make([]*AccessLogEntry, len(lines))
	var parseErrors int64 // Use atomic operations for error counting

	// Channels for work distribution
	lineChan := make(chan lineWork, len(lines))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.parseWorkerOptimized(lineChan, results, &parseErrors)
		}()
	}

	// Send work to workers
	go func() {
		defer close(lineChan)
		for i, line := range lines {
			lineChan <- lineWork{index: i, line: line}
		}
	}()

	// Wait for workers to complete
	wg.Wait()

	// Filter out nil entries and build final result - O(n) single pass
	entries := make([]*AccessLogEntry, 0, len(lines))
	for _, entry := range results {
		if entry != nil {
			entries = append(entries, entry)
		}
	}

	if parseErrors > 3 {
		logger.Warnf("Total parse errors: %d out of %d lines", parseErrors, len(lines))
	}

	logger.Debugf("Successfully parsed %d entries out of %d lines (%d parse errors) using %d workers",
		len(entries), len(lines), parseErrors, numWorkers)

	return entries
}

// parseLinesSingleThreaded uses the original single-threaded parsing logic
func (p *LogParser) parseLinesSingleThreaded(lines []string) []*AccessLogEntry {
	var entries []*AccessLogEntry
	var parseErrors int

	for i, line := range lines {
		if entry, err := p.ParseLine(line); err == nil && entry != nil {
			entries = append(entries, entry)
		} else if err != nil {
			parseErrors++
			if parseErrors <= 3 {
				logger.Debugf("Failed to parse log line %d: %v, line: %s", i+1, err, line)
			}
		}
	}

	if parseErrors > 3 {
		logger.Warnf("Total parse errors: %d out of %d lines", parseErrors, len(lines))
	}

	logger.Infof("Successfully parsed %d entries out of %d lines (%d parse errors) - single-threaded",
		len(entries), len(lines), parseErrors)

	return entries
}

// lineWork represents work to be done by a parser worker
type lineWork struct {
	index int
	line  string
}

// parseResult represents the result of parsing a line
type parseResult struct {
	index int
	line  string
	entry *AccessLogEntry
	err   error
}

// parseWorker processes lines from the work channel (legacy version)
func (p *LogParser) parseWorker(lineChan <-chan lineWork, resultChan chan<- parseResult) {
	for work := range lineChan {
		entry, err := p.ParseLine(work.line)
		resultChan <- parseResult{
			index: work.index,
			line:  work.line,
			entry: entry,
			err:   err,
		}
	}
}

// parseWorkerOptimized processes lines and directly writes to pre-allocated array - O(1) insertion
func (p *LogParser) parseWorkerOptimized(lineChan <-chan lineWork, results []*AccessLogEntry, parseErrors *int64) {
	var localErrors int64

	for work := range lineChan {
		entry, err := p.ParseLine(work.line)

		// Direct insertion at correct index - no synchronization needed since each index is unique
		results[work.index] = entry

		// Count errors locally to reduce atomic contention
		if err != nil {
			localErrors++
			if localErrors <= 3 {
				logger.Debugf("Failed to parse log line %d: %v, line: %s", work.index+1, err, work.line)
			}
		}
	}

	// Update global error count once per worker using atomic operation
	if localErrors > 0 {
		atomic.AddInt64(parseErrors, localErrors)
	}
}

// DetectLogFormat attempts to detect the log format used
func DetectLogFormat(sampleLines []string) *LogFormat {
	formatScores := make(map[string]int)

	for _, line := range sampleLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		for _, format := range SupportedFormats {
			if format.Pattern.MatchString(line) {
				formatScores[format.Name]++
			}
		}
	}

	// Find the format with the highest score
	var bestFormat *LogFormat
	var bestScore int

	for _, format := range SupportedFormats {
		if score := formatScores[format.Name]; score > bestScore {
			bestScore = score
			bestFormat = format
		}
	}

	return bestFormat
}
