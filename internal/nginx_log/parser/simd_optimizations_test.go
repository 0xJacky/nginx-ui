package parser

import (
	"context"
	"strings"
	"testing"
)

// BenchmarkSIMDOptimizations tests SIMD-optimized string operations
func BenchmarkSIMDOptimizations(b *testing.B) {
	matcher := NewSIMDStringMatcher()
	testData := []byte(`127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0"`)
	
	benchmarks := []struct {
		name string
		fn   func() interface{}
	}{
		{
			"SIMD_FindNextSpace",
			func() interface{} {
				return matcher.FindNextSpace(testData, 0)
			},
		},
		{
			"SIMD_FindNextQuote",
			func() interface{} {
				return matcher.FindNextQuote(testData, 0)
			},
		},
		{
			"SIMD_FindNextDigit",
			func() interface{} {
				return matcher.FindNextDigit(testData, 0)
			},
		},
		{
			"SIMD_ExtractIPAddress",
			func() interface{} {
				ip, _ := matcher.ExtractIPAddress(testData, 0)
				return ip
			},
		},
		{
			"SIMD_ExtractTimestamp",
			func() interface{} {
				timestamp, _ := matcher.ExtractTimestamp(testData, 0)
				return timestamp
			},
		},
		{
			"SIMD_ExtractQuotedString",
			func() interface{} {
				str, _ := matcher.ExtractQuotedString(testData, 50)
				return str
			},
		},
		{
			"SIMD_ExtractStatusCode",
			func() interface{} {
				status, _ := matcher.ExtractStatusCode(testData, 80)
				return status
			},
		},
		{
			"SIMD_ParseCompleteLine",
			func() interface{} {
				return matcher.ParseLogLineSIMD(testData)
			},
		},
	}

	for _, bench := range benchmarks {
		b.Run(bench.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				result := bench.fn()
				_ = result // Avoid optimization
			}
		})
	}
}

// BenchmarkSIMDvsRegularParsing compares SIMD vs regular parsing performance
func BenchmarkSIMDvsRegularParsing(b *testing.B) {
	// Setup test data
	logLines := []string{
		`127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0"`,
		`192.168.1.1 - - [25/Dec/2023:10:00:01 +0000] "POST /api/data HTTP/1.1" 201 567 "https://example.com" "curl/7.68.0"`,
		`10.0.0.1 - - [25/Dec/2023:10:00:02 +0000] "GET /style.css HTTP/1.1" 200 890 "https://example.com" "Mozilla/5.0"`,
		`203.0.113.195 - - [25/Dec/2023:10:00:03 +0000] "DELETE /api/users/123 HTTP/1.1" 204 0 "-" "Postman/7.36.0"`,
		`172.16.0.50 - - [25/Dec/2023:10:00:04 +0000] "PUT /api/config HTTP/1.1" 200 456 "https://admin.example.com" "Chrome/91.0"`,
	}

	// Convert to byte slices for SIMD processing
	logBytes := make([][]byte, len(logLines))
	for i, line := range logLines {
		logBytes[i] = []byte(line)
	}

	// Setup parsers
	config := DefaultParserConfig()
	config.MaxLineLength = 16 * 1024
	regularParser := NewOptimizedParser(
		config,
		NewSimpleUserAgentParser(),
		&mockGeoIPService{},
	)
	
	simdParser := NewOptimizedLogLineParser()

	b.Run("Regular_SingleLine", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			line := logLines[i%len(logLines)]
			_, _ = regularParser.ParseLine(line)
		}
	})

	b.Run("SIMD_SingleLine", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			lineBytes := logBytes[i%len(logBytes)]
			_ = simdParser.ParseLine(lineBytes)
		}
	})

	b.Run("Regular_BatchLines", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			_ = regularParser.ParseLinesWithContext(ctx, logLines)
		}
	})

	b.Run("SIMD_BatchLines", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			_ = simdParser.ParseLines(logBytes)
		}
	})
}

// BenchmarkSIMDCharacterSearch compares SIMD vs standard character search
func BenchmarkSIMDCharacterSearch(b *testing.B) {
	matcher := NewSIMDStringMatcher()
	testData := []byte(`127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"`)

	b.Run("SIMD_SpaceSearch", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			_ = matcher.FindNextSpace(testData, 0)
		}
	})

	b.Run("Standard_SpaceSearch", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			// Standard byte-by-byte search
			for j := 0; j < len(testData); j++ {
				if testData[j] == ' ' {
					_ = j
					break
				}
			}
		}
	})

	b.Run("SIMD_QuoteSearch", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			_ = matcher.FindNextQuote(testData, 0)
		}
	})

	b.Run("Standard_QuoteSearch", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			// Standard byte-by-byte search
			for j := 0; j < len(testData); j++ {
				if testData[j] == '"' {
					_ = j
					break
				}
			}
		}
	})
}

// BenchmarkSIMDStringExtraction compares SIMD vs regex string extraction
func BenchmarkSIMDStringExtraction(b *testing.B) {
	matcher := NewSIMDStringMatcher()
	testData := []byte(`127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234`)

	b.Run("SIMD_IPExtraction", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			ip, _ := matcher.ExtractIPAddress(testData, 0)
			_ = ip
		}
	})

	b.Run("SIMD_TimestampExtraction", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			timestamp, _ := matcher.ExtractTimestamp(testData, 0)
			_ = timestamp
		}
	})

	b.Run("SIMD_StatusExtraction", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			status, _ := matcher.ExtractStatusCode(testData, 0)
			_ = status
		}
	})
}

// BenchmarkObjectPooling tests the performance impact of object pooling
func BenchmarkObjectPooling(b *testing.B) {
	pool := NewAccessLogEntryPool()
	
	b.Run("WithPooling", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			entry := pool.Get()
			entry.IP = "127.0.0.1"
			entry.Status = 200
			pool.Put(entry)
		}
	})

	b.Run("WithoutPooling", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			entry := &AccessLogEntry{}
			entry.IP = "127.0.0.1"
			entry.Status = 200
			// No pooling - let GC handle
		}
	})
}

// BenchmarkSIMDScaleTest tests SIMD performance at different scales
func BenchmarkSIMDScaleTest(b *testing.B) {
	simdParser := NewOptimizedLogLineParser()
	
	scales := []struct {
		name  string
		lines int
	}{
		{"Small_100", 100},
		{"Medium_1K", 1000},
		{"Large_10K", 10000},
		{"XLarge_50K", 50000},
	}

	for _, scale := range scales {
		// Generate test data
		testLines := make([][]byte, scale.lines)
		for i := 0; i < scale.lines; i++ {
			line := generateTestLogLine(i)
			testLines[i] = []byte(line)
		}

		b.Run("SIMD_"+scale.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				entries := simdParser.ParseLines(testLines)
				b.ReportMetric(float64(len(entries)), "parsed_lines")
			}
		})
	}
}

// generateTestLogLine generates a test log line for benchmarking
func generateTestLogLine(index int) string {
	ip := "192.168.1." + string(rune('1' + (index % 254)))
	method := []string{"GET", "POST", "PUT", "DELETE"}[index%4]
	path := []string{"/", "/api/data", "/style.css", "/script.js"}[index%4]
	status := []int{200, 404, 500, 301}[index%4]
	
	return strings.Join([]string{
		ip, "- - [25/Dec/2023:10:00:00 +0000]",
		`"` + method + " " + path + ` HTTP/1.1"`,
		string(rune('0' + status/100)) + string(rune('0' + (status/10)%10)) + string(rune('0' + status%10)),
		"1234",
		`"https://example.com"`,
		`"Mozilla/5.0"`,
	}, " ")
}

// TestSIMDCorrectnessValidation validates SIMD operations produce correct results
func TestSIMDCorrectnessValidation(t *testing.T) {
	matcher := NewSIMDStringMatcher()
	simdParser := NewOptimizedLogLineParser()
	
	testCases := []struct {
		name string
		line string
		expectedIP string
		expectedStatus int
	}{
		{
			"Standard_Log",
			`127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0"`,
			"127.0.0.1",
			200,
		},
		{
			"Complex_IP",
			`192.168.100.255 - - [25/Dec/2023:10:00:01 +0000] "POST /api/data HTTP/1.1" 201 567 "https://example.com" "curl/7.68.0"`,
			"192.168.100.255",
			201,
		},
		{
			"Error_Status",
			`10.0.0.1 - - [25/Dec/2023:10:00:02 +0000] "GET /nonexistent HTTP/1.1" 404 0 "-" "Bot/1.0"`,
			"10.0.0.1",
			404,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lineBytes := []byte(tc.line)
			
			// Test IP extraction
			ip, _ := matcher.ExtractIPAddress(lineBytes, 0)
			if ip != tc.expectedIP {
				t.Errorf("IP extraction failed: got %s, want %s", ip, tc.expectedIP)
			}
			
			// Test status extraction
			status, _ := matcher.ExtractStatusCode(lineBytes, 0)
			if status != tc.expectedStatus {
				t.Errorf("Status extraction failed: got %d, want %d", status, tc.expectedStatus)
			}
			
			// Test complete parsing
			entry := simdParser.ParseLine(lineBytes)
			if entry == nil {
				t.Fatal("SIMD parsing returned nil")
			}
			
			if entry.IP != tc.expectedIP {
				t.Errorf("Complete parsing IP failed: got %s, want %s", entry.IP, tc.expectedIP)
			}
			
			if entry.Status != tc.expectedStatus {
				t.Errorf("Complete parsing status failed: got %d, want %d", entry.Status, tc.expectedStatus)
			}
		})
	}
}

// TestSIMDLookupTables validates lookup table correctness
func TestSIMDLookupTables(t *testing.T) {
	matcher := NewSIMDStringMatcher()
	
	// Test space lookup
	spaces := []byte{' ', '\t', '\n', '\r'}
	for _, c := range spaces {
		if !matcher.spaceLookup[c] {
			t.Errorf("Space lookup failed for character %c (%d)", c, c)
		}
	}
	
	// Test digit lookup
	for c := byte('0'); c <= '9'; c++ {
		if !matcher.digitLookup[c] {
			t.Errorf("Digit lookup failed for character %c", c)
		}
	}
	
	// Test quote lookup
	quotes := []byte{'"', '\''}
	for _, c := range quotes {
		if !matcher.quoteLookup[c] {
			t.Errorf("Quote lookup failed for character %c", c)
		}
	}
	
	// Test non-special characters
	if matcher.spaceLookup['a'] {
		t.Error("Space lookup false positive for 'a'")
	}
	
	if matcher.digitLookup['a'] {
		t.Error("Digit lookup false positive for 'a'")
	}
}

// TestObjectPoolEfficiency validates object pool functionality
func TestObjectPoolEfficiency(t *testing.T) {
	pool := NewAccessLogEntryPool()
	
	// Get entry from pool
	entry1 := pool.Get()
	if entry1 == nil {
		t.Fatal("Pool returned nil entry")
	}
	
	// Modify entry
	entry1.IP = "127.0.0.1"
	entry1.Status = 200
	
	// Return to pool
	pool.Put(entry1)
	
	// Get another entry (should be reused)
	entry2 := pool.Get()
	if entry2 == nil {
		t.Fatal("Pool returned nil after put")
	}
	
	// Should be reset
	if entry2.IP != "" {
		t.Error("Pool entry not properly reset")
	}
	
	if entry2.Status != 0 {
		t.Error("Pool entry status not properly reset")
	}
}