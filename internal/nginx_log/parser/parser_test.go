package parser

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Mock implementations for testing
type mockGeoIPService struct{}

func (m *mockGeoIPService) Search(ip string) (*GeoLocation, error) {
	if ip == "127.0.0.1" || ip == "::1" {
		return &GeoLocation{
			RegionCode: "US",
			Province:   "California",
			City:       "San Francisco",
		}, nil
	}
	return &GeoLocation{
		RegionCode: "US",
		Province:   "Unknown",
		City:       "Unknown",
	}, nil
}

func TestOptimizedParser_ParseLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantErr  bool
		validate func(*AccessLogEntry) bool
	}{
		{
			name: "combined log format",
			line: `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"`,
			validate: func(entry *AccessLogEntry) bool {
				return entry.IP == "127.0.0.1" &&
					entry.Method == "GET" &&
					entry.Path == "/index.html" &&
					entry.Status == 200 &&
					entry.BytesSent == 1234
			},
		},
		{
			name: "with request and upstream time",
			line: `192.168.1.1 - - [25/Dec/2023:10:00:00 +0000] "POST /api/data HTTP/1.1" 201 567 "-" "curl/7.68.0" 0.123 0.045`,
			validate: func(entry *AccessLogEntry) bool {
				return entry.IP == "192.168.1.1" &&
					entry.Method == "POST" &&
					entry.Status == 201 &&
					entry.RequestTime == 0.123 &&
					entry.UpstreamTime != nil &&
					*entry.UpstreamTime == 0.045
			},
		},
		{
			name:    "empty line",
			line:    "",
			wantErr: true,
		},
		{
			name:    "malformed line",
			line:    "not a valid log line",
			wantErr: false, // Non-strict mode should handle this gracefully
			validate: func(entry *AccessLogEntry) bool {
				return entry.Raw == "not a valid log line"
			},
		},
		{
			name: "minimal valid line",
			line: `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET / HTTP/1.1" 200 -`,
			validate: func(entry *AccessLogEntry) bool {
				return entry.IP == "127.0.0.1" &&
					entry.Method == "GET" &&
					entry.Path == "/" &&
					entry.Status == 200 &&
					entry.BytesSent == 0
			},
		},
	}

	config := DefaultParserConfig()
	config.StrictMode = false // Use non-strict mode to handle malformed lines gracefully
	
	parser := NewOptimizedParser(
		config,
		NewSimpleUserAgentParser(),
		&mockGeoIPService{},
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.ParseLine(tt.line)
			
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if entry == nil {
				t.Error("expected entry but got nil")
				return
			}
			
			if tt.validate != nil && !tt.validate(entry) {
				t.Errorf("entry validation failed: %+v", entry)
			}
			
			// Verify common fields
			if entry.ID == "" {
				t.Error("entry ID should not be empty")
			}
			
			if entry.Raw != tt.line {
				t.Errorf("raw line mismatch: got %q, want %q", entry.Raw, tt.line)
			}
		})
	}
}

func TestOptimizedParser_ParseLines(t *testing.T) {
	lines := []string{
		`127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "-" "Mozilla/5.0"`,
		`192.168.1.1 - - [25/Dec/2023:10:00:01 +0000] "POST /api/data HTTP/1.1" 201 567 "-" "curl/7.68.0"`,
		`10.0.0.1 - - [25/Dec/2023:10:00:02 +0000] "GET /style.css HTTP/1.1" 200 890 "-" "Mozilla/5.0"`,
		"", // empty line
		"invalid log line", // malformed line
	}

	config := DefaultParserConfig()
	parser := NewOptimizedParser(
		config,
		NewSimpleUserAgentParser(),
		&mockGeoIPService{},
	)

	result := parser.ParseLines(lines)

	if result.Processed != len(lines) {
		t.Errorf("processed count mismatch: got %d, want %d", result.Processed, len(lines))
	}

	if result.Succeeded != 4 {
		t.Errorf("success count mismatch: got %d, want 4", result.Succeeded)
	}

	if result.Failed != 1 {
		t.Errorf("failure count mismatch: got %d, want 1", result.Failed)
	}

	if len(result.Entries) != 4 {
		t.Errorf("entries count mismatch: got %d, want 4", len(result.Entries))
	}
}

func TestOptimizedParser_ParseStream(t *testing.T) {
	logData := `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "-" "Mozilla/5.0"
192.168.1.1 - - [25/Dec/2023:10:00:01 +0000] "POST /api/data HTTP/1.1" 201 567 "-" "curl/7.68.0"
10.0.0.1 - - [25/Dec/2023:10:00:02 +0000] "GET /style.css HTTP/1.1" 200 890 "-" "Mozilla/5.0"`

	reader := strings.NewReader(logData)
	
	config := DefaultParserConfig()
	config.BatchSize = 2 // Small batch size for testing
	
	parser := NewOptimizedParser(
		config,
		NewSimpleUserAgentParser(),
		&mockGeoIPService{},
	)

	ctx := context.Background()
	result, err := parser.ParseStream(ctx, reader)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.Processed != 3 {
		t.Errorf("processed count mismatch: got %d, want 3", result.Processed)
	}

	if result.Succeeded != 3 {
		t.Errorf("success count mismatch: got %d, want 3", result.Succeeded)
	}

	if len(result.Entries) != 3 {
		t.Errorf("entries count mismatch: got %d, want 3", len(result.Entries))
	}
}

func TestOptimizedParser_WithContext(t *testing.T) {
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = fmt.Sprintf(`127.0.0.%d - - [25/Dec/2023:10:00:00 +0000] "GET /test%d.html HTTP/1.1" 200 1234 "-" "Mozilla/5.0"`, i%255+1, i)
	}

	config := DefaultParserConfig()
	parser := NewOptimizedParser(
		config,
		NewSimpleUserAgentParser(),
		&mockGeoIPService{},
	)

	// Test with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result := parser.ParseLinesWithContext(ctx, lines)

	// Should either complete or be cancelled
	if result.Processed == 0 {
		t.Error("no lines were processed")
	}
}

func TestFormatDetector(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		expected string
	}{
		{
			name: "combined format",
			lines: []string{
				`127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"`,
				`192.168.1.1 - - [25/Dec/2023:10:00:01 +0000] "POST /api/data HTTP/1.1" 201 567 "-" "curl/7.68.0"`,
			},
			expected: "combined",
		},
		{
			name: "main format",
			lines: []string{
				`127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234`,
				`192.168.1.1 - - [25/Dec/2023:10:00:01 +0000] "POST /api/data HTTP/1.1" 201 567`,
			},
			expected: "main",
		},
		{
			name: "no match",
			lines: []string{
				"completely invalid log format",
				"another invalid line",
			},
			expected: "",
		},
	}

	detector := NewFormatDetector()
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format := detector.DetectFormat(tt.lines)
			
			if tt.expected == "" {
				if format != nil {
					t.Errorf("expected no format detection, but got %s", format.Name)
				}
			} else {
				if format == nil {
					t.Errorf("expected format %s, but got nil", tt.expected)
				} else if format.Name != tt.expected {
					t.Errorf("expected format %s, but got %s", tt.expected, format.Name)
				}
			}
		})
	}
}

func TestSimpleUserAgentParser(t *testing.T) {
	parser := NewSimpleUserAgentParser()
	
	tests := []struct {
		name      string
		userAgent string
		expected  UserAgentInfo
	}{
		{
			name:      "Chrome on Windows",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.110 Safari/537.36",
			expected: UserAgentInfo{
				Browser:    "Chrome",
				BrowserVer: "96.0",
				OS:         "Windows",
				OSVersion:  "10.0",
				DeviceType: "Desktop",
			},
		},
		{
			name:      "Firefox on macOS",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:95.0) Gecko/20100101 Firefox/95.0",
			expected: UserAgentInfo{
				Browser:    "Firefox",
				BrowserVer: "95.0",
				OS:         "macOS",
				OSVersion:  "10.15",
				DeviceType: "Desktop",
			},
		},
		{
			name:      "Mobile Safari on iOS",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 15_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.2 Mobile/15E148 Safari/604.1",
			expected: UserAgentInfo{
				Browser:    "Safari",
				BrowserVer: "15.2",
				OS:         "iOS",
				OSVersion:  "15.2",
				DeviceType: "iPhone",
			},
		},
		{
			name:      "Empty user agent",
			userAgent: "-",
			expected: UserAgentInfo{
				Browser:    "Unknown",
				BrowserVer: "",
				OS:         "Unknown",
				OSVersion:  "",
				DeviceType: "Desktop",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.userAgent)
			
			if result.Browser != tt.expected.Browser {
				t.Errorf("browser mismatch: got %s, want %s", result.Browser, tt.expected.Browser)
			}
			
			if result.OS != tt.expected.OS {
				t.Errorf("OS mismatch: got %s, want %s", result.OS, tt.expected.OS)
			}
			
			if result.DeviceType != tt.expected.DeviceType {
				t.Errorf("device type mismatch: got %s, want %s", result.DeviceType, tt.expected.DeviceType)
			}
		})
	}
}

func TestCachedUserAgentParser(t *testing.T) {
	baseParser := NewSimpleUserAgentParser()
	cachedParser := NewCachedUserAgentParser(baseParser, 5)
	
	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/96.0.4664.110"
	
	// First parse should cache the result
	result1 := cachedParser.Parse(userAgent)
	
	// Second parse should use cached result
	result2 := cachedParser.Parse(userAgent)
	
	if result1.Browser != result2.Browser {
		t.Error("cached result differs from original")
	}
	
	size, maxSize := cachedParser.GetCacheStats()
	if size != 1 {
		t.Errorf("expected cache size 1, got %d", size)
	}
	
	if maxSize != 5 {
		t.Errorf("expected max cache size 5, got %d", maxSize)
	}
}

func BenchmarkOptimizedParser_ParseLine(b *testing.B) {
	line := `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"`
	
	config := DefaultParserConfig()
	parser := NewOptimizedParser(
		config,
		NewSimpleUserAgentParser(),
		&mockGeoIPService{},
	)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := parser.ParseLine(line)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOptimizedParser_ParseLines(b *testing.B) {
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = fmt.Sprintf(`127.0.0.%d - - [25/Dec/2023:10:00:00 +0000] "GET /test%d.html HTTP/1.1" 200 1234 "-" "Mozilla/5.0"`, i%255+1, i)
	}
	
	config := DefaultParserConfig()
	parser := NewOptimizedParser(
		config,
		NewSimpleUserAgentParser(),
		&mockGeoIPService{},
	)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		result := parser.ParseLines(lines)
		if result.Failed > 0 {
			b.Fatalf("parsing failed: %d errors", result.Failed)
		}
	}
}