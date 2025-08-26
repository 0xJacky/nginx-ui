package indexer

import (
	"strconv"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/geolite"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/parser"
	"github.com/uozi-tech/cosy/logger"
)

// Global parser instance
var (
	logParser *parser.OptimizedParser // Use the concrete type
)

func init() {
	// Initialize the parser with all its dependencies during package initialization.
	uaParser := parser.NewSimpleUserAgentParser()

	var geoIPService parser.GeoIPService
	geoService, err := geolite.GetService()
	if err != nil {
		logger.Warnf("Failed to initialize GeoIP service, geo-enrichment will be disabled: %v", err)
	} else {
		geoIPService = parser.NewGeoLiteAdapter(geoService)
	}

	// Create the optimized parser with real dependencies.
	logParser = parser.NewOptimizedParser(nil, uaParser, geoIPService)
}

// ParseLogLine parses a raw log line into a structured LogDocument
func ParseLogLine(line string) (*LogDocument, error) {
	entry, err := logParser.ParseLine(line)
	if err != nil {
		return nil, err
	}

	// Convert parser.AccessLogEntry to indexer.LogDocument
	// This mapping is necessary because the indexer and parser might have different data structures.
	logDoc := &LogDocument{
		Timestamp:   entry.Timestamp,
		IP:          entry.IP,
		RegionCode:  entry.RegionCode,
		Province:    entry.Province,
		City:        entry.City,
		Method:      entry.Method,
		Path:        entry.Path,
		PathExact:   entry.Path, // Use the same for now
		Protocol:    entry.Protocol,
		Status:      entry.Status,
		BytesSent:   entry.BytesSent,
		Referer:     entry.Referer,
		UserAgent:   entry.UserAgent,
		Browser:     entry.Browser,
		BrowserVer:  entry.BrowserVer,
		OS:          entry.OS,
		OSVersion:   entry.OSVersion,
		DeviceType:  entry.DeviceType,
		RequestTime: entry.RequestTime,
		Raw:         entry.Raw,
	}

	if entry.UpstreamTime != nil {
		logDoc.UpstreamTime = entry.UpstreamTime
	}

	return logDoc, nil
}

// Quick parse for request field "GET /path HTTP/1.1"
func parseRequestField(request string) (method, path, protocol string) {
	parts := strings.Split(request, " ")
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2]
	}
	return "UNKNOWN", request, "UNKNOWN"
}

// Quick parse for timestamp, e.g., "02/Jan/2006:15:04:05 -0700"
func parseTimestamp(ts string) int64 {
	t, err := time.Parse("02/Jan/2006:15:04:05 -0700", ts)
	if err != nil {
		return 0
	}
	return t.Unix()
}

// Quick string to int64 conversion
func toInt64(s string) int64 {
	val, _ := strconv.ParseInt(s, 10, 64)
	return val
}

// Quick string to int conversion
func toInt(s string) int {
	val, _ := strconv.Atoi(s)
	return val
}
