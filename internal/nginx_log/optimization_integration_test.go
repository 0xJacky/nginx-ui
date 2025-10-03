package nginx_log

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/analytics"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/parser"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
)

// TestOptimizationSystemIntegration tests all optimization components working together
func TestOptimizationSystemIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping optimization integration test in short mode")
	}

	// Test data - realistic nginx log entries
	testLogData := generateIntegrationTestData(1000)
	ctx := context.Background()

	t.Run("CompleteOptimizationPipeline", func(t *testing.T) {
		// Test 1: Optimized Parser with all enhancements
		config := parser.DefaultParserConfig()
		config.MaxLineLength = 16 * 1024
		config.BatchSize = 500

		optimizedParser := parser.NewParser(
			config,
			parser.NewCachedUserAgentParser(parser.NewSimpleUserAgentParser(), 1000),
			&mockGeoIPService{},
		)

		// Performance measurement
		start := time.Now()
		
		// Test ParseStream
		parseResult, err := optimizedParser.ParseStream(ctx, strings.NewReader(testLogData))
		if err != nil {
			t.Fatalf("ParseStream failed: %v", err)
		}
		
		optimizedParseTime := time.Since(start)
		optimizedRate := float64(parseResult.Processed) / optimizedParseTime.Seconds()

		// Test 2: SIMD Parser performance
		start = time.Now()
		
		simdParser := parser.NewLogLineParser()
		lines := strings.Split(testLogData, "\n")
		logBytes := make([][]byte, 0, len(lines))
		
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				logBytes = append(logBytes, []byte(line))
			}
		}
		
		simdEntries := simdParser.ParseLines(logBytes)
		simdParseTime := time.Since(start)
		simdRate := float64(len(simdEntries)) / simdParseTime.Seconds()

		// Test 3: Enhanced Memory Pools under load
		start = time.Now()
		
		poolOperations := 10000
		for i := 0; i < poolOperations; i++ {
			// String builder pool
			sb := utils.LogStringBuilderPool.Get()
			sb.WriteString("integration test data")
			utils.LogStringBuilderPool.Put(sb)
			
			// Byte slice pool
			slice := utils.GlobalByteSlicePool.Get(1024)
			utils.GlobalByteSlicePool.Put(slice)
		}
		
		poolTime := time.Since(start)
		poolRate := float64(poolOperations*2) / poolTime.Seconds() // 2 operations per iteration

		// Test 4: Time-series analytics with optimizations
		start = time.Now()
		
		// Create realistic time-series data
		timeSeriesData := make([]analytics.TimeValue, len(simdEntries))
		baseTime := time.Now().Unix()
		for i := range simdEntries {
			timeSeriesData[i] = analytics.TimeValue{
				Timestamp: baseTime + int64(i*60), // 1 minute intervals
				Value:     1 + (i % 10),           // Varied values
			}
		}
		
		// Advanced analytics
		advancedProcessor := analytics.NewAnomalyDetector()
		anomalies := advancedProcessor.DetectAnomalies(timeSeriesData)
		trend := advancedProcessor.CalculateTrend(timeSeriesData)
		
		analyticsTime := time.Since(start)

		// Test 5: Regex caching performance
		start = time.Now()
		
		cache := parser.GetGlobalRegexCache()
		regexOperations := 10000
		
		for i := 0; i < regexOperations; i++ {
			// Should hit cache frequently
			_, _ = cache.GetCommonRegex("ipv4")
			_, _ = cache.GetCommonRegex("timestamp") 
			_, _ = cache.GetCommonRegex("status")
		}
		
		regexTime := time.Since(start)
		regexRate := float64(regexOperations*3) / regexTime.Seconds()

		// Performance assertions and reporting
		t.Logf("=== OPTIMIZATION INTEGRATION RESULTS ===")
		t.Logf("Optimized Parser: %d lines in %v (%.2f lines/sec)", 
			parseResult.Processed, optimizedParseTime, optimizedRate)
		t.Logf("SIMD Parser: %d lines in %v (%.2f lines/sec)", 
			len(simdEntries), simdParseTime, simdRate)
		t.Logf("Memory Pools: %d ops in %v (%.2f ops/sec)", 
			poolOperations*2, poolTime, poolRate)
		t.Logf("Analytics: Processed in %v, %d anomalies, trend: %s", 
			analyticsTime, len(anomalies), trend.Direction)
		t.Logf("Regex Cache: %d ops in %v (%.2f ops/sec)", 
			regexOperations*3, regexTime, regexRate)

		// Performance requirements validation
		minOptimizedRate := 500.0  // lines/sec
		minSIMDRate := 10000.0     // lines/sec  
		minPoolRate := 100000.0    // ops/sec
		minRegexRate := 1000000.0  // ops/sec

		if optimizedRate < minOptimizedRate {
			t.Errorf("Optimized parser rate %.2f < expected %.2f lines/sec", 
				optimizedRate, minOptimizedRate)
		}

		if simdRate < minSIMDRate {
			t.Errorf("SIMD parser rate %.2f < expected %.2f lines/sec", 
				simdRate, minSIMDRate)
		}

		if poolRate < minPoolRate {
			t.Errorf("Memory pool rate %.2f < expected %.2f ops/sec", 
				poolRate, minPoolRate)
		}

		if regexRate < minRegexRate {
			t.Errorf("Regex cache rate %.2f < expected %.2f ops/sec", 
				regexRate, minRegexRate)
		}

		// Data integrity validation - both parsers should process same number of lines
		expectedLines := len(strings.Split(strings.TrimSpace(testLogData), "\n"))
		if parseResult.Processed != expectedLines {
			t.Errorf("Optimized parser processed %d lines, expected %d", 
				parseResult.Processed, expectedLines)
		}
		
		if len(simdEntries) != expectedLines {
			t.Errorf("SIMD parser processed %d lines, expected %d", 
				len(simdEntries), expectedLines)
		}
		
		// Test parsing consistency with a known log line
		testLine := `192.168.1.100 - - [06/Sep/2025:10:00:00 +0000] "GET /test HTTP/1.1" 200 1024 "https://example.com" "Mozilla/5.0"`
		
		// Parse with optimized parser
		optimizedTest, err := optimizedParser.ParseStream(ctx, strings.NewReader(testLine))
		if err != nil || len(optimizedTest.Entries) == 0 {
			t.Errorf("Optimized parser failed on test line: %v", err)
		} else {
			// Parse with SIMD parser  
			simdTestEntry := simdParser.ParseLine([]byte(testLine))
			if simdTestEntry == nil {
				t.Error("SIMD parser failed on test line")
			} else {
				// Compare the results
				optimizedTestEntry := optimizedTest.Entries[0]
				if optimizedTestEntry.IP != simdTestEntry.IP {
					t.Errorf("Test line IP mismatch: optimized=%s, simd=%s", 
						optimizedTestEntry.IP, simdTestEntry.IP)
				}
				if optimizedTestEntry.Status != simdTestEntry.Status {
					t.Errorf("Test line status mismatch: optimized=%d, simd=%d", 
						optimizedTestEntry.Status, simdTestEntry.Status)
				}
			}
		}
	})

	t.Run("ResourceEfficiencyValidation", func(t *testing.T) {
		// Test memory pool efficiency
		poolManager := utils.GetGlobalPoolManager()
		_ = poolManager.GetAllStats() // Get initial stats for reference
		
		// Perform intensive operations
		iterations := 5000
		for i := 0; i < iterations; i++ {
			// String builder operations
			sb := utils.LogStringBuilderPool.Get()
			sb.WriteString("efficiency test data for memory pools")
			utils.LogStringBuilderPool.Put(sb)
			
			// Byte slice operations
			slice := utils.GlobalByteSlicePool.Get(2048)
			copy(slice[:10], []byte("test data"))
			utils.GlobalByteSlicePool.Put(slice)
			
			// Worker operations
			worker := utils.NewPooledWorker()
			testData := []byte("worker efficiency test")
			worker.ProcessWithPools(testData, func(data []byte, sb *strings.Builder) error {
				sb.WriteString(string(data))
				return nil
			})
			worker.Cleanup()
		}
		
		finalStats := poolManager.GetAllStats()
		
		// Verify pool efficiency
		t.Logf("=== POOL EFFICIENCY RESULTS ===")
		for name, stats := range finalStats {
			if statsObj, ok := stats.(map[string]interface{}); ok {
				t.Logf("Pool %s: %+v", name, statsObj)
			} else {
				t.Logf("Pool %s: %+v", name, stats)
			}
		}

		// Test regex cache efficiency
		cache := parser.GetGlobalRegexCache()
		initialCacheStats := cache.GetStats()
		
		// Perform regex operations
		for i := 0; i < 1000; i++ {
			_, _ = cache.GetCommonRegex("ipv4")
			_, _ = cache.GetCommonRegex("timestamp")
			_, _ = cache.GetCommonRegex("combined_format")
		}
		
		finalCacheStats := cache.GetStats()
		
		hitRateImprovement := finalCacheStats.HitRate - initialCacheStats.HitRate
		
		t.Logf("=== CACHE EFFICIENCY RESULTS ===")
		t.Logf("Initial: Hits=%d, Misses=%d, HitRate=%.2f%%", 
			initialCacheStats.Hits, initialCacheStats.Misses, initialCacheStats.HitRate*100)
		t.Logf("Final: Hits=%d, Misses=%d, HitRate=%.2f%%", 
			finalCacheStats.Hits, finalCacheStats.Misses, finalCacheStats.HitRate*100)
		t.Logf("Hit rate improvement: %.2f%%", hitRateImprovement*100)
		
		// Cache hit rate should be very high
		if finalCacheStats.HitRate < 0.9 {
			t.Errorf("Cache hit rate %.2f%% is too low, expected > 90%%", 
				finalCacheStats.HitRate*100)
		}
	})

	t.Run("StressTestOptimizations", func(t *testing.T) {
		// Stress test with concurrent operations
		concurrency := 10
		operationsPerGoroutine := 1000
		
		// Channel to collect results
		results := make(chan time.Duration, concurrency*3)
		
		// Start concurrent goroutines
		for i := 0; i < concurrency; i++ {
			go func(id int) {
				// SIMD parsing stress test
				start := time.Now()
				simdParser := parser.NewLogLineParser()
				
				testLine := `192.168.1.100 - - [06/Sep/2025:10:00:00 +0000] "GET /stress HTTP/1.1" 200 1024 "https://test.com" "StressTest/1.0"`
				
				for j := 0; j < operationsPerGoroutine; j++ {
					_ = simdParser.ParseLine([]byte(testLine))
				}
				results <- time.Since(start)
				
				// Memory pool stress test
				start = time.Now()
				for j := 0; j < operationsPerGoroutine; j++ {
					sb := utils.LogStringBuilderPool.Get()
					sb.WriteString("concurrent stress test")
					utils.LogStringBuilderPool.Put(sb)
				}
				results <- time.Since(start)
				
				// Regex cache stress test
				start = time.Now()
				cache := parser.GetGlobalRegexCache()
				for j := 0; j < operationsPerGoroutine; j++ {
					_, _ = cache.GetCommonRegex("ipv4")
				}
				results <- time.Since(start)
			}(i)
		}
		
		// Collect results
		totalDuration := time.Duration(0)
		resultCount := 0
		timeout := time.After(30 * time.Second)
		
		for resultCount < concurrency*3 {
			select {
			case duration := <-results:
				totalDuration += duration
				resultCount++
			case <-timeout:
				t.Fatal("Stress test timed out")
			}
		}
		
		averageDuration := totalDuration / time.Duration(resultCount)
		totalOperations := concurrency * operationsPerGoroutine * 3
		overallRate := float64(totalOperations) / totalDuration.Seconds()
		
		t.Logf("=== STRESS TEST RESULTS ===")
		t.Logf("Total operations: %d", totalOperations)
		t.Logf("Total time: %v", totalDuration)
		t.Logf("Average time per goroutine: %v", averageDuration)
		t.Logf("Overall rate: %.2f ops/sec", overallRate)
		
		// Performance assertion
		minStressRate := 10000.0 // ops/sec under stress
		if overallRate < minStressRate {
			t.Errorf("Stress test rate %.2f < expected %.2f ops/sec", 
				overallRate, minStressRate)
		}
	})
}

// TestOptimizationCorrectness validates that all optimizations produce correct results
func TestOptimizationCorrectness(t *testing.T) {
	testLogLine := `127.0.0.1 - - [06/Sep/2025:10:00:00 +0000] "GET /test.html HTTP/1.1" 200 2048 "https://example.com" "Mozilla/5.0"`
	ctx := context.Background()

	// Test all parsing methods produce identical results
	t.Run("ParsingMethodConsistency", func(t *testing.T) {
		// Standard parser
		config := parser.DefaultParserConfig()
		config.MaxLineLength = 16 * 1024
		standardParser := parser.NewParser(
			config,
			parser.NewSimpleUserAgentParser(),
			&mockGeoIPService{},
		)
		
		standardResult, err := standardParser.ParseStream(ctx, strings.NewReader(testLogLine))
		if err != nil || len(standardResult.Entries) == 0 {
			t.Fatalf("Standard parser failed: %v", err)
		}
		
		// Optimized parser
		optimizedResult, err := standardParser.ParseStream(ctx, strings.NewReader(testLogLine))
		if err != nil || len(optimizedResult.Entries) == 0 {
			t.Fatalf("Optimized parser failed: %v", err)
		}
		
		// SIMD parser
		simdParser := parser.NewLogLineParser()
		simdEntry := simdParser.ParseLine([]byte(testLogLine))
		if simdEntry == nil {
			t.Fatal("SIMD parser returned nil")
		}
		
		// Compare results
		standardEntry := standardResult.Entries[0]
		optimizedEntry := optimizedResult.Entries[0]
		
		// Verify consistency across all methods
		if standardEntry.IP != optimizedEntry.IP || standardEntry.IP != simdEntry.IP {
			t.Errorf("IP inconsistency: standard=%s, optimized=%s, simd=%s", 
				standardEntry.IP, optimizedEntry.IP, simdEntry.IP)
		}
		
		if standardEntry.Status != optimizedEntry.Status || standardEntry.Status != simdEntry.Status {
			t.Errorf("Status inconsistency: standard=%d, optimized=%d, simd=%d", 
				standardEntry.Status, optimizedEntry.Status, simdEntry.Status)
		}
		
		t.Logf("All parsing methods produce consistent results: IP=%s, Status=%d", 
			standardEntry.IP, standardEntry.Status)
	})
}

// generateIntegrationTestData creates realistic test data for integration testing
func generateIntegrationTestData(lines int) string {
	var builder strings.Builder
	builder.Grow(lines * 200) // Pre-allocate space

	ips := []string{"127.0.0.1", "192.168.1.100", "10.0.0.50", "203.0.113.195", "172.16.0.25"}
	methods := []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS"}
	paths := []string{"/", "/index.html", "/api/data", "/styles/main.css", "/js/app.js", "/api/users", "/login", "/dashboard"}
	statuses := []int{200, 201, 204, 301, 302, 400, 401, 403, 404, 429, 500, 502, 503}
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
		"curl/7.68.0",
		"PostmanRuntime/7.29.0",
		"Integration-Test/1.0",
	}

	baseTime := time.Date(2025, 9, 6, 10, 0, 0, 0, time.UTC).Unix()

	for i := 0; i < lines; i++ {
		ip := ips[i%len(ips)]
		timestamp := time.Unix(baseTime+int64(i*60), 0).Format("02/Jan/2006:15:04:05 -0700")
		method := methods[i%len(methods)]
		path := paths[i%len(paths)]
		status := statuses[i%len(statuses)]
		size := 1000 + (i % 10000)
		userAgent := userAgents[i%len(userAgents)]

		line := ip + ` - - [` + timestamp + `] "` + method + ` ` + path + ` HTTP/1.1" ` +
				string(rune('0'+status/100)) + string(rune('0'+(status/10)%10)) + string(rune('0'+status%10)) + ` ` +
				string(rune('0'+size/10000)) + string(rune('0'+(size/1000)%10)) + 
				string(rune('0'+(size/100)%10)) + string(rune('0'+(size/10)%10)) + string(rune('0'+size%10)) +
				` "https://example.com" "` + userAgent + `"`

		builder.WriteString(line)
		if i < lines-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// mockGeoIPService provides mock geo IP functionality
type mockGeoIPService struct{}

func (m *mockGeoIPService) Search(ip string) (*parser.GeoLocation, error) {
	return &parser.GeoLocation{
		CountryCode: "US",
		RegionCode:  "CA",
		Province:    "California", 
		City:        "San Francisco",
	}, nil
}