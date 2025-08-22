package parser

import (
	"regexp"
)

// Common nginx log formats
var (
	// Standard combined log format
	CombinedFormat = &LogFormat{
		Name:    "combined",
		Pattern: regexp.MustCompile(`^(\S+) - (\S+) \[([^]]+)\] "([^"]*)" (\d+) (\d+|-) "([^"]*)" "([^"]*)"(?:\s+(\S+))?(?:\s+(\S+))?`),
		Fields:  []string{"ip", "remote_user", "timestamp", "request", "status", "bytes_sent", "referer", "user_agent", "request_time", "upstream_time"},
	}

	// Standard main log format (common log format)
	MainFormat = &LogFormat{
		Name:    "main",
		Pattern: regexp.MustCompile(`^(\S+) - (\S+) \[([^]]+)\] "([^"]*)" (\d+) (\d+|-)(?:\s+"([^"]*)")?(?:\s+"([^"]*)")?`),
		Fields:  []string{"ip", "remote_user", "timestamp", "request", "status", "bytes_sent", "referer", "user_agent"},
	}

	// Custom format with more details
	DetailedFormat = &LogFormat{
		Name:    "detailed",
		Pattern: regexp.MustCompile(`^(\S+) - (\S+) \[([^]]+)\] "([^"]*)" (\d+) (\d+|-) "([^"]*)" "([^"]*)" (\S+) (\S+) "([^"]*)" (\S+)`),
		Fields:  []string{"ip", "remote_user", "timestamp", "request", "status", "bytes_sent", "referer", "user_agent", "request_time", "upstream_time", "x_forwarded_for", "connection"},
	}

	// All supported formats ordered by priority
	SupportedFormats = []*LogFormat{DetailedFormat, CombinedFormat, MainFormat}
)

// FormatDetector handles automatic log format detection
type FormatDetector struct {
	formats       []*LogFormat
	sampleSize    int
	matchThreshold float64
}

// NewFormatDetector creates a new format detector
func NewFormatDetector() *FormatDetector {
	return &FormatDetector{
		formats:        SupportedFormats,
		sampleSize:     100,
		matchThreshold: 0.8, // 80% match rate required
	}
}

// DetectFormat tries to detect the log format from sample lines
func (fd *FormatDetector) DetectFormat(lines []string) *LogFormat {
	if len(lines) == 0 {
		return nil
	}

	sampleLines := lines
	if len(lines) > fd.sampleSize {
		sampleLines = lines[:fd.sampleSize]
	}

	for _, format := range fd.formats {
		matchCount := 0
		for _, line := range sampleLines {
			if format.Pattern.MatchString(line) {
				matchCount++
			}
		}
		
		matchRate := float64(matchCount) / float64(len(sampleLines))
		if matchRate >= fd.matchThreshold {
			return format
		}
	}

	return nil
}

// DetectFormatWithDetails returns detailed detection results
func (fd *FormatDetector) DetectFormatWithDetails(lines []string) (*LogFormat, map[string]float64) {
	if len(lines) == 0 {
		return nil, nil
	}

	sampleLines := lines
	if len(lines) > fd.sampleSize {
		sampleLines = lines[:fd.sampleSize]
	}

	results := make(map[string]float64)
	var bestFormat *LogFormat
	var bestScore float64

	for _, format := range fd.formats {
		matchCount := 0
		for _, line := range sampleLines {
			if format.Pattern.MatchString(line) {
				matchCount++
			}
		}
		
		score := float64(matchCount) / float64(len(sampleLines))
		results[format.Name] = score
		
		if score > bestScore {
			bestScore = score
			bestFormat = format
		}
	}

	if bestScore >= fd.matchThreshold {
		return bestFormat, results
	}

	return nil, results
}

// AddCustomFormat adds a custom log format to the detector
func (fd *FormatDetector) AddCustomFormat(format *LogFormat) {
	fd.formats = append([]*LogFormat{format}, fd.formats...)
}

// SetMatchThreshold sets the minimum match rate required for format detection
func (fd *FormatDetector) SetMatchThreshold(threshold float64) {
	if threshold > 0 && threshold <= 1 {
		fd.matchThreshold = threshold
	}
}

// SetSampleSize sets the number of lines to use for format detection
func (fd *FormatDetector) SetSampleSize(size int) {
	if size > 0 {
		fd.sampleSize = size
	}
}