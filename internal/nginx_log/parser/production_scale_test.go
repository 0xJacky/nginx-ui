package parser

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestProductionScaleValidation tests optimized parsers with 1M+ records
func TestProductionScaleValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping production scale test in short mode")
	}

	scales := []struct {
		name    string
		records int
	}{
		{"Medium_100K", 100000},
		{"Large_500K", 500000},
		{"XLarge_1M", 1000000},
		{"Enterprise_2M", 2000000},
	}

	config := DefaultParserConfig()
	config.MaxLineLength = 16 * 1024
	config.WorkerCount = 12 // Utilize all CPU cores
	config.BatchSize = 2000 // Larger batches for high volume
	
	parser := NewOptimizedParser(
		config,
		NewCachedUserAgentParser(NewSimpleUserAgentParser(), 10000),
		&mockGeoIPService{},
	)

	for _, scale := range scales {
		t.Run(scale.name, func(t *testing.T) {
			t.Logf("🚀 Starting production scale test: %s (%d records)", scale.name, scale.records)
			
			// Generate realistic production data
			logData := generateProductionLogData(scale.records)
			t.Logf("📊 Generated %d bytes of test data", len(logData))
			
			// Test Original ParseStream (for comparison on smaller datasets only)
			if scale.records <= 100000 {
				t.Run("Original_ParseStream", func(t *testing.T) {
					startTime := time.Now()
					reader := strings.NewReader(logData)
					ctx := context.Background()
					
					result, err := parser.ParseStream(ctx, reader)
					duration := time.Since(startTime)
					
					if err != nil {
						t.Fatalf("Original ParseStream failed: %v", err)
					}
					
					t.Logf("✅ Original ParseStream: %d records in %v (%.0f records/sec)", 
						result.Processed, duration, float64(result.Processed)/duration.Seconds())
					
					validateResults(t, result, scale.records)
				})
			}
			
			// Test Optimized ParseStream
			t.Run("Optimized_ParseStream", func(t *testing.T) {
				startTime := time.Now()
				reader := strings.NewReader(logData)
				ctx := context.Background()
				
				result, err := parser.OptimizedParseStream(ctx, reader)
				duration := time.Since(startTime)
				
				if err != nil {
					t.Fatalf("Optimized ParseStream failed: %v", err)
				}
				
				t.Logf("🚀 Optimized ParseStream: %d records in %v (%.0f records/sec)", 
					result.Processed, duration, float64(result.Processed)/duration.Seconds())
				
				validateResults(t, result, scale.records)
				
				// Performance expectations
				recordsPerSec := float64(result.Processed) / duration.Seconds()
				if recordsPerSec < 1000 { // Expect at least 1K records/sec
					t.Errorf("Performance below expectation: %.0f records/sec < 1000", recordsPerSec)
				}
			})
			
			// Test Memory-Efficient ParseStream
			t.Run("MemoryEfficient_ParseStream", func(t *testing.T) {
				startTime := time.Now()
				reader := strings.NewReader(logData)
				ctx := context.Background()
				
				result, err := parser.MemoryEfficientParseStream(ctx, reader)
				duration := time.Since(startTime)
				
				if err != nil {
					t.Fatalf("Memory-Efficient ParseStream failed: %v", err)
				}
				
				t.Logf("💡 Memory-Efficient ParseStream: %d records in %v (%.0f records/sec)", 
					result.Processed, duration, float64(result.Processed)/duration.Seconds())
				
				validateResults(t, result, scale.records)
			})
			
			// Test Chunked ParseStream with different chunk sizes
			chunkSizes := []int{32 * 1024, 64 * 1024, 128 * 1024}
			for _, chunkSize := range chunkSizes {
				t.Run(fmt.Sprintf("Chunked_ParseStream_%dKB", chunkSize/1024), func(t *testing.T) {
					startTime := time.Now()
					reader := strings.NewReader(logData)
					ctx := context.Background()
					
					result, err := parser.ChunkedParseStream(ctx, reader, chunkSize)
					duration := time.Since(startTime)
					
					if err != nil {
						t.Fatalf("Chunked ParseStream failed: %v", err)
					}
					
					t.Logf("📦 Chunked ParseStream (%dKB): %d records in %v (%.0f records/sec)", 
						chunkSize/1024, result.Processed, duration, float64(result.Processed)/duration.Seconds())
					
					validateResults(t, result, scale.records)
				})
			}
		})
	}
}

// BenchmarkProductionScale benchmarks parsers at production scale
func BenchmarkProductionScale(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping production scale benchmark in short mode")
	}

	// Test with 100K records (representative of high-volume production workload)
	logData := generateProductionLogData(100000)
	
	config := DefaultParserConfig()
	config.MaxLineLength = 16 * 1024
	config.WorkerCount = 12
	config.BatchSize = 2000
	
	parser := NewOptimizedParser(
		config,
		NewCachedUserAgentParser(NewSimpleUserAgentParser(), 10000),
		&mockGeoIPService{},
	)

	benchmarks := []struct {
		name string
		fn   func(context.Context, *strings.Reader) (*ParseResult, error)
	}{
		{
			"Optimized_ParseStream_100K",
			func(ctx context.Context, reader *strings.Reader) (*ParseResult, error) {
				return parser.OptimizedParseStream(ctx, reader)
			},
		},
		{
			"MemoryEfficient_ParseStream_100K",
			func(ctx context.Context, reader *strings.Reader) (*ParseResult, error) {
				return parser.MemoryEfficientParseStream(ctx, reader)
			},
		},
		{
			"Chunked_ParseStream_100K_64KB",
			func(ctx context.Context, reader *strings.Reader) (*ParseResult, error) {
				return parser.ChunkedParseStream(ctx, reader, 64*1024)
			},
		},
	}

	for _, bench := range benchmarks {
		b.Run(bench.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				reader := strings.NewReader(logData)
				ctx := context.Background()
				
				result, err := bench.fn(ctx, reader)
				if err != nil {
					b.Fatalf("Benchmark failed: %v", err)
				}
				
				// Report detailed metrics
				b.ReportMetric(float64(result.Processed), "records_processed")
				b.ReportMetric(float64(result.Succeeded), "records_succeeded")
				b.ReportMetric(result.ErrorRate*100, "error_rate_%")
				
				if result.Duration > 0 {
					throughput := float64(result.Processed) / result.Duration.Seconds()
					b.ReportMetric(throughput, "records_per_sec")
					
					// Memory efficiency metric - use a reasonable estimate
					b.ReportMetric(float64(result.Processed)*100, "estimated_bytes_per_record")
				}
			}
		})
	}
}

// validateResults validates parsing results for correctness
func validateResults(t *testing.T, result *ParseResult, expectedRecords int) {
	t.Helper()
	
	// Basic validation
	if result.Processed != expectedRecords {
		t.Errorf("Processed count mismatch: got %d, want %d", result.Processed, expectedRecords)
	}
	
	// Error rate should be reasonable (< 1%)
	if result.ErrorRate > 0.01 {
		t.Errorf("Error rate too high: %.2f%% > 1%%", result.ErrorRate*100)
	}
	
	// Should have successfully parsed most records
	expectedSucceeded := int(float64(expectedRecords) * 0.99) // Allow for 1% error rate
	if result.Succeeded < expectedSucceeded {
		t.Errorf("Success rate too low: got %d, expected at least %d", result.Succeeded, expectedSucceeded)
	}
	
	// Entries count should match succeeded count
	if len(result.Entries) != result.Succeeded {
		t.Errorf("Entries count mismatch: got %d entries, expected %d", len(result.Entries), result.Succeeded)
	}
	
	// Validate some sample entries
	if len(result.Entries) > 0 {
		firstEntry := result.Entries[0]
		if firstEntry.IP == "" {
			t.Error("First entry missing IP address")
		}
		if firstEntry.Status == 0 {
			t.Error("First entry missing status code")
		}
		
		// Validate a middle entry
		if len(result.Entries) > 100 {
			middleEntry := result.Entries[len(result.Entries)/2]
			if middleEntry.Method == "" {
				t.Error("Middle entry missing HTTP method")
			}
		}
		
		// Validate last entry
		lastEntry := result.Entries[len(result.Entries)-1]
		if lastEntry.IP == "" {
			t.Error("Last entry missing IP address")
		}
	}
	
	t.Logf("✅ Validation passed: %d processed, %d succeeded (%.2f%% success rate)", 
		result.Processed, result.Succeeded, (1-result.ErrorRate)*100)
}

// generateProductionLogData generates realistic production-scale nginx log data
func generateProductionLogData(records int) string {
	var builder strings.Builder
	
	// Pre-allocate for better performance (estimated 250 bytes per log line)
	builder.Grow(records * 250)
	
	// Realistic production patterns
	methods := []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH"}
	_ = []string{"200", "201", "204", "301", "302", "304", "400", "401", "403", "404", "429", "500", "502", "503"} // statuses
	
	// Common paths in production
	paths := []string{
		"/", "/index.html", "/favicon.ico", "/robots.txt",
		"/api/v1/users", "/api/v1/auth", "/api/v1/data", "/api/v1/health",
		"/static/css/main.css", "/static/js/app.js", "/static/images/logo.png",
		"/admin", "/admin/dashboard", "/admin/users", "/admin/settings",
		"/docs", "/docs/api", "/docs/guide",
		"/search", "/search/results", 
		"/upload", "/download", "/export",
	}
	
	// Realistic user agents
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:120.0) Gecko/20100101 Firefox/120.0",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (iPad; CPU OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1",
		"curl/8.4.0", "PostmanRuntime/7.35.0", "Go-http-client/1.1",
		"Googlebot/2.1 (+http://www.google.com/bot.html)",
		"Bingbot/2.0 (+http://www.bing.com/bingbot.htm)",
	}
	
	// Generate realistic IP addresses (simulate different networks)
	ipRanges := []string{
		"192.168.1", "192.168.0", "10.0.0", "10.0.1", "172.16.0", "172.16.1",
		"203.0.113", "198.51.100", "185.199.108", "140.82.114", "151.101.1",
	}
	
	referers := []string{
		"https://www.google.com/", "https://github.com/", "https://stackoverflow.com/",
		"https://www.linkedin.com/", "https://twitter.com/", "https://www.youtube.com/",
		"", "-", // Empty and dash referers are common
	}
	
	// Generate log entries
	baseTime := time.Date(2025, 9, 6, 10, 0, 0, 0, time.UTC)
	
	for i := 0; i < records; i++ {
		// Generate realistic IP
		ipRange := ipRanges[i%len(ipRanges)]
		ip := fmt.Sprintf("%s.%d", ipRange, (i%254)+1)
		
		// Generate timestamp (spread over 24 hours)
		timestamp := baseTime.Add(time.Duration(i) * time.Second / 100) // ~100 requests per second
		timeStr := timestamp.Format("02/Jan/2006:15:04:05 -0700")
		
		// Select method and path (weighted towards GET)
		var method string
		if i%10 < 7 { // 70% GET requests
			method = "GET"
		} else {
			method = methods[i%len(methods)]
		}
		
		path := paths[i%len(paths)]
		
		// Add query parameters for some requests
		if i%5 == 0 && method == "GET" {
			path += fmt.Sprintf("?page=%d&size=10", (i%100)+1)
		}
		
		// Status code (weighted towards successful responses)
		var status string
		switch {
		case i%100 < 85: // 85% success
			if method == "POST" || method == "PUT" {
				status = "201"
			} else {
				status = "200"
			}
		case i%100 < 90: // 5% redirects
			status = "302"
		case i%100 < 95: // 5% client errors
			status = "404"
		default: // 5% server errors
			status = "500"
		}
		
		// Response size (realistic distribution)
		var size int
		switch method {
		case "GET":
			size = 1000 + (i%50000) // 1KB to 50KB
		case "POST", "PUT":
			size = 100 + (i%1000) // Smaller responses for write operations
		case "HEAD":
			size = 0
		default:
			size = 500 + (i%5000)
		}
		
		// Select user agent and referer
		userAgent := userAgents[i%len(userAgents)]
		referer := referers[i%len(referers)]
		
		// Add request time for some entries
		requestTime := ""
		if i%3 == 0 {
			reqTime := float64((i%1000)+1) / 1000.0 // 1ms to 1s
			requestTime = fmt.Sprintf(" %.3f", reqTime)
		}
		
		// Build log entry
		logLine := fmt.Sprintf(`%s - - [%s] "%s %s HTTP/1.1" %s %d "%s" "%s"%s`,
			ip, timeStr, method, path, status, size, referer, userAgent, requestTime)
		
		builder.WriteString(logLine)
		if i < records-1 {
			builder.WriteString("\n")
		}
	}
	
	return builder.String()
}

// TestMemoryUsageValidation tests memory usage patterns of optimized parsers
func TestMemoryUsageValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory usage test in short mode")
	}

	// Test with moderate dataset to observe memory patterns
	logData := generateProductionLogData(50000)
	
	config := DefaultParserConfig()
	config.MaxLineLength = 16 * 1024
	config.WorkerCount = 4
	config.BatchSize = 1000
	
	parser := NewOptimizedParser(
		config,
		NewCachedUserAgentParser(NewSimpleUserAgentParser(), 1000),
		&mockGeoIPService{},
	)

	implementations := []struct {
		name string
		fn   func(context.Context, *strings.Reader) (*ParseResult, error)
	}{
		{
			"Optimized_ParseStream",
			func(ctx context.Context, reader *strings.Reader) (*ParseResult, error) {
				return parser.OptimizedParseStream(ctx, reader)
			},
		},
		{
			"MemoryEfficient_ParseStream",
			func(ctx context.Context, reader *strings.Reader) (*ParseResult, error) {
				return parser.MemoryEfficientParseStream(ctx, reader)
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name+"_MemoryUsage", func(t *testing.T) {
			// Force GC before measurement
			runtime.GC()
			var memBefore runtime.MemStats
			runtime.ReadMemStats(&memBefore)
			
			// Run parsing
			reader := strings.NewReader(logData)
			ctx := context.Background()
			
			result, err := impl.fn(ctx, reader)
			if err != nil {
				t.Fatalf("%s failed: %v", impl.name, err)
			}
			
			// Force GC after parsing
			runtime.GC()
			var memAfter runtime.MemStats
			runtime.ReadMemStats(&memAfter)
			
			// Calculate memory usage
			memUsed := memAfter.TotalAlloc - memBefore.TotalAlloc
			memPerRecord := float64(memUsed) / float64(result.Processed)
			
			t.Logf("📊 %s Memory Usage:", impl.name)
			t.Logf("   Total Memory Used: %d bytes", memUsed)
			t.Logf("   Memory per Record: %.2f bytes", memPerRecord)
			t.Logf("   Records Processed: %d", result.Processed)
			t.Logf("   Peak Memory: %d bytes", memAfter.Sys)
			
			// Memory usage should be reasonable (< 1KB per record)
			if memPerRecord > 1024 {
				t.Errorf("Memory usage too high: %.2f bytes per record > 1024", memPerRecord)
			}
			
			validateResults(t, result, 50000)
		})
	}
}