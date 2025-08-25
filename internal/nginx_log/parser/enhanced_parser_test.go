package parser

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Additional comprehensive performance benchmarks
func BenchmarkOptimizedParser_ParseStream(b *testing.B) {
	logData := strings.Repeat(`127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0"`+"\n", 1000)

	config := DefaultParserConfig()
	parser := NewOptimizedParser(
		config,
		NewSimpleUserAgentParser(),
		&mockGeoIPService{},
	)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(logData)
		ctx := context.Background()
		_, err := parser.ParseStream(ctx, reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOptimizedParser_LargeScale(b *testing.B) {
	lines := make([]string, 10000)
	for i := range lines {
		lines[i] = fmt.Sprintf(`192.168.%d.%d - - [25/Dec/2023:10:%02d:%02d +0000] "GET /api/data/%d HTTP/1.1" 200 %d "https://example.com/page%d" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/96.%d.%d.%d"`,
			i%256, (i/256)%256, (i/60)%60, i%60, i, 1000+i, i%100, i%100, (i*7)%100, i%1000)
	}

	config := DefaultParserConfig()
	config.WorkerCount = 4
	config.BatchSize = 1000
	parser := NewOptimizedParser(
		config,
		NewCachedUserAgentParser(NewSimpleUserAgentParser(), 1000),
		&mockGeoIPService{},
	)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		result := parser.ParseLinesWithContext(ctx, lines)
		if result.Failed > 0 {
			b.Fatalf("parsing failed: %d errors", result.Failed)
		}
	}
}

func BenchmarkUserAgentParsing(b *testing.B) {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.110 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.110 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.110 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 15_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.2 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Android 11; Mobile; rv:95.0) Gecko/95.0 Firefox/95.0",
	}

	b.Run("Simple", func(b *testing.B) {
		parser := NewSimpleUserAgentParser()
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			userAgent := userAgents[i%len(userAgents)]
			parser.Parse(userAgent)
		}
	})

	b.Run("Cached", func(b *testing.B) {
		parser := NewCachedUserAgentParser(NewSimpleUserAgentParser(), 100)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			userAgent := userAgents[i%len(userAgents)]
			parser.Parse(userAgent)
		}
	})
}

func BenchmarkConcurrentParsing(b *testing.B) {
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = fmt.Sprintf(`127.0.0.%d - - [25/Dec/2023:10:00:00 +0000] "GET /test%d.html HTTP/1.1" 200 1234 "-" "Mozilla/5.0"`, i%255+1, i)
	}

	config := DefaultParserConfig()
	config.WorkerCount = 8
	parser := NewOptimizedParser(
		config,
		NewCachedUserAgentParser(NewSimpleUserAgentParser(), 100),
		&mockGeoIPService{},
	)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result := parser.ParseLines(lines[:100]) // Smaller batches for parallel test
			if result.Failed > 0 {
				b.Fatalf("parsing failed: %d errors", result.Failed)
			}
		}
	})
}

// Memory usage benchmarks
func BenchmarkMemoryUsage(b *testing.B) {
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
		entry, err := parser.ParseLine(line)
		if err != nil {
			b.Fatal(err)
		}
		_ = entry // Prevent optimization
	}
}

// Edge case tests
func TestOptimizedParser_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantErr  bool
		validate func(*AccessLogEntry) bool
	}{
		{
			name: "IPv6 address",
			line: `2001:0db8:85a3:0000:0000:8a2e:0370:7334 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234`,
			validate: func(entry *AccessLogEntry) bool {
				return entry.IP == "2001:0db8:85a3:0000:0000:8a2e:0370:7334"
			},
		},
		{
			name: "Very long path",
			line: `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /` + strings.Repeat("a", 2000) + ` HTTP/1.1" 200 1234`,
			validate: func(entry *AccessLogEntry) bool {
				return len(entry.Path) == 2001 && strings.HasPrefix(entry.Path, "/a")
			},
		},
		{
			name: "Special characters in path",
			line: `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /测试/path%20with%20spaces?param=value&other=测试 HTTP/1.1" 200 1234`,
			validate: func(entry *AccessLogEntry) bool {
				return strings.Contains(entry.Path, "测试") && strings.Contains(entry.Path, "spaces")
			},
		},
		{
			name: "Large response size",
			line: `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /large-file HTTP/1.1" 200 999999999999`,
			validate: func(entry *AccessLogEntry) bool {
				return entry.BytesSent == 999999999999
			},
		},
		{
			name: "HTTP/2 protocol",
			line: `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/2" 200 1234`,
			validate: func(entry *AccessLogEntry) bool {
				return entry.Protocol == "HTTP/2"
			},
		},
		{
			name: "Extreme timing values",
			line: `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /slow HTTP/1.1" 200 1234 "-" "Mozilla/5.0" 30.123456 45.987654`,
			validate: func(entry *AccessLogEntry) bool {
				return entry.RequestTime == 30.123456 && entry.UpstreamTime != nil && *entry.UpstreamTime == 45.987654
			},
		},
	}

	config := DefaultParserConfig()
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
		})
	}
}

// Concurrent safety test
func TestOptimizedParser_ConcurrentSafety(t *testing.T) {
	config := DefaultParserConfig()
	parser := NewOptimizedParser(
		config,
		NewCachedUserAgentParser(NewSimpleUserAgentParser(), 100),
		&mockGeoIPService{},
	)

	lines := make([]string, 100)
	for i := range lines {
		lines[i] = fmt.Sprintf(`127.0.0.%d - - [25/Dec/2023:10:00:00 +0000] "GET /test%d.html HTTP/1.1" 200 1234 "-" "Mozilla/5.0"`, i%255+1, i)
	}

	// Start multiple goroutines parsing simultaneously
	const numGoroutines = 10
	results := make(chan *ParseResult, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			result := parser.ParseLines(lines)
			results <- result
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		result := <-results
		if result.Failed > 0 {
			t.Errorf("parsing failed in goroutine: %d errors", result.Failed)
		}
		if result.Succeeded != 100 {
			t.Errorf("expected 100 successful parses, got %d", result.Succeeded)
		}
	}
}

// Cache performance tests
func TestCachedUserAgentParser_Performance(t *testing.T) {
	baseParser := NewSimpleUserAgentParser()
	cachedParser := NewCachedUserAgentParser(baseParser, 10)

	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/96.0.4664.110"

	// Fill cache
	for i := 0; i < 5; i++ {
		uaVariant := fmt.Sprintf("%s.%d", userAgent, i)
		cachedParser.Parse(uaVariant)
	}

	// Test cache hits
	start := time.Now()
	for i := 0; i < 1000; i++ {
		uaVariant := fmt.Sprintf("%s.%d", userAgent, i%5)
		cachedParser.Parse(uaVariant)
	}
	cacheTime := time.Since(start)

	// Test without cache
	start = time.Now()
	for i := 0; i < 1000; i++ {
		uaVariant := fmt.Sprintf("%s.%d", userAgent, i%5)
		baseParser.Parse(uaVariant)
	}
	baseTime := time.Since(start)

	// Cache should be significantly faster
	if cacheTime >= baseTime {
		t.Logf("Cache time: %v, Base time: %v", cacheTime, baseTime)
		t.Error("cached parser should be faster than base parser for repeated queries")
	}

	size, _ := cachedParser.GetCacheStats()
	if size != 5 {
		t.Errorf("expected cache size 5, got %d", size)
	}
}

// Stress test with malformed data
func TestOptimizedParser_StressTest(t *testing.T) {
	config := DefaultParserConfig()
	config.StrictMode = false
	parser := NewOptimizedParser(
		config,
		NewSimpleUserAgentParser(),
		&mockGeoIPService{},
	)

	// Generate mix of valid and invalid log lines
	lines := make([]string, 1000)
	for i := range lines {
		switch i % 10 {
		case 0:
			lines[i] = "" // Empty line
		case 1:
			lines[i] = "totally invalid log line" // Completely invalid
		case 2:
			lines[i] = `incomplete log line - - [25/Dec/2023:10:00:00` // Incomplete
		case 3:
			lines[i] = `127.0.0.1 - - [invalid-date] "GET / HTTP/1.1" 200 1234` // Invalid date
		default:
			// Valid lines with variations
			lines[i] = fmt.Sprintf(`192.168.%d.%d - - [25/Dec/2023:10:%02d:%02d +0000] "GET /test%d HTTP/1.1" %d %d "-" "Mozilla/5.0"`,
				i%256, (i/256)%256, (i/60)%60, i%60, i, 200+(i%100), 1000+i)
		}
	}

	result := parser.ParseLines(lines)

	// Should handle all lines gracefully
	if result.Processed != len(lines) {
		t.Errorf("processed count mismatch: got %d, want %d", result.Processed, len(lines))
	}

	// Should have some failures for malformed lines
	if result.Failed == 0 {
		t.Error("expected some parsing failures for malformed lines")
	}

	// Should have majority successes
	if float64(result.Succeeded)/float64(result.Processed) < 0.6 {
		t.Errorf("success rate too low: %d/%d = %.2f%%", result.Succeeded, result.Processed, 100.0*float64(result.Succeeded)/float64(result.Processed))
	}
}

// Test resource cleanup
func TestOptimizedParser_ResourceCleanup(t *testing.T) {
	config := DefaultParserConfig()
	config.WorkerCount = 4
	parser := NewOptimizedParser(
		config,
		NewCachedUserAgentParser(NewSimpleUserAgentParser(), 100),
		&mockGeoIPService{},
	)

	// Create many parsing operations to test resource management
	for i := 0; i < 10; i++ {
		lines := make([]string, 100)
		for j := range lines {
			lines[j] = fmt.Sprintf(`127.0.0.%d - - [25/Dec/2023:10:00:00 +0000] "GET /test%d.html HTTP/1.1" 200 1234 "-" "Mozilla/5.0"`, j%255+1, j)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		result := parser.ParseLinesWithContext(ctx, lines)
		cancel()

		if result.Failed > 0 {
			t.Errorf("iteration %d: unexpected parsing failures: %d", i, result.Failed)
		}
	}
}

// Performance comparison between different configurations
func BenchmarkParserConfigurations(b *testing.B) {
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = fmt.Sprintf(`127.0.0.%d - - [25/Dec/2023:10:00:00 +0000] "GET /test%d.html HTTP/1.1" 200 1234 "-" "Mozilla/5.0"`, i%255+1, i)
	}

	configs := []struct {
		name   string
		config *Config
	}{
		{
			name: "Single Worker",
			config: &Config{
				WorkerCount: 1,
				BatchSize:   100,
				BufferSize:  1000,
				EnableGeoIP: false,
				StrictMode:  false,
			},
		},
		{
			name: "Multiple Workers",
			config: &Config{
				WorkerCount: 4,
				BatchSize:   250,
				BufferSize:  2000,
				EnableGeoIP: false,
				StrictMode:  false,
			},
		},
		{
			name: "With GeoIP",
			config: &Config{
				WorkerCount: 4,
				BatchSize:   250,
				BufferSize:  2000,
				EnableGeoIP: true,
				StrictMode:  false,
			},
		},
		{
			name: "Strict Mode",
			config: &Config{
				WorkerCount: 4,
				BatchSize:   250,
				BufferSize:  2000,
				EnableGeoIP: false,
				StrictMode:  true,
			},
		},
	}

	for _, cfg := range configs {
		b.Run(cfg.name, func(b *testing.B) {
			parser := NewOptimizedParser(
				cfg.config,
				NewCachedUserAgentParser(NewSimpleUserAgentParser(), 100),
				&mockGeoIPService{},
			)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				result := parser.ParseLines(lines)
				if result.Failed > len(lines)/2 { // Allow some failures in strict mode
					b.Fatalf("too many parsing failures: %d", result.Failed)
				}
			}
		})
	}
}
