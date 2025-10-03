package parser

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// BenchmarkParseStreamComparison compares different ParseStream implementations
func BenchmarkParseStreamComparison(b *testing.B) {
	// Generate test data - 1000 lines of realistic log data
	logData := generateBenchmarkLogData(1000)

	config := DefaultParserConfig()
	config.MaxLineLength = 16 * 1024
	parser := NewParser(
		config,
		NewCachedUserAgentParser(NewSimpleUserAgentParser(), 100),
		&mockGeoIPService{},
	)

	benchmarks := []struct {
		name string
		fn   func(context.Context, *strings.Reader) (*ParseResult, error)
	}{
		{
			name: "Original_ParseStream",
			fn: func(ctx context.Context, reader *strings.Reader) (*ParseResult, error) {
				return parser.ParseStream(ctx, reader)
			},
		},
		{
			name: "Optimized_ParseStream",
			fn: func(ctx context.Context, reader *strings.Reader) (*ParseResult, error) {
				return parser.ParseStream(ctx, reader)
			},
		},
		{
			name: "Chunked_ParseStream",
			fn: func(ctx context.Context, reader *strings.Reader) (*ParseResult, error) {
				return parser.ChunkedParseStream(ctx, reader, 32*1024)
			},
		},
		{
			name: "MemoryEfficient_ParseStream",
			fn: func(ctx context.Context, reader *strings.Reader) (*ParseResult, error) {
				return parser.MemoryEfficientParseStream(ctx, reader)
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
					b.Fatalf("Parse error: %v", err)
				}

				if result.Processed == 0 {
					b.Fatal("No lines processed")
				}

				// Report custom metrics
				b.ReportMetric(float64(result.Processed), "lines_processed")
				b.ReportMetric(float64(result.Succeeded), "lines_succeeded")
				b.ReportMetric(result.ErrorRate*100, "error_rate_%")
				if result.Duration > 0 {
					throughput := float64(result.Processed) / result.Duration.Seconds()
					b.ReportMetric(throughput, "lines_per_sec")
				}
			}
		})
	}
}

// BenchmarkStreamParsing_ScaleTest tests parsing performance at different scales
func BenchmarkStreamParsing_ScaleTest(b *testing.B) {
	scales := []struct {
		name  string
		lines int
	}{
		{"Small_100", 100},
		{"Medium_1K", 1000},
		{"Large_10K", 10000},
		{"XLarge_50K", 50000},
	}

	config := DefaultParserConfig()
	config.MaxLineLength = 16 * 1024
	parser := NewParser(
		config,
		NewCachedUserAgentParser(NewSimpleUserAgentParser(), 1000),
		&mockGeoIPService{},
	)

	for _, scale := range scales {
		logData := generateBenchmarkLogData(scale.lines)

		b.Run("Original_"+scale.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				reader := strings.NewReader(logData)
				ctx := context.Background()

				result, err := parser.ParseStream(ctx, reader)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(result.Processed), "lines")
			}
		})

		b.Run("Optimized_"+scale.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				reader := strings.NewReader(logData)
				ctx := context.Background()

				result, err := parser.ParseStream(ctx, reader)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(result.Processed), "lines")
			}
		})
	}
}

// BenchmarkStreamOptimizations_Individual tests individual optimization components
func BenchmarkStreamOptimizations_Individual(b *testing.B) {
	_ = generateBenchmarkLogData(5000) // Avoid unused warning

	config := DefaultParserConfig()
	config.MaxLineLength = 16 * 1024
	_ = NewParser(
		config,
		NewSimpleUserAgentParser(),
		&mockGeoIPService{},
	) // Avoid unused warning

	b.Run("UnsafeStringConversion", func(b *testing.B) {
		testBytes := []byte("127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] \"GET /index.html HTTP/1.1\" 200 1234")

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = unsafeBytesToString(testBytes)
		}
	})

	b.Run("StandardStringConversion", func(b *testing.B) {
		testBytes := []byte("127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] \"GET /index.html HTTP/1.1\" 200 1234")

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = string(testBytes)
		}
	})

	b.Run("LineBuffer_Operations", func(b *testing.B) {
		buffer := NewLineBuffer(1024)
		testData := []byte("test log line data")

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			buffer.Reset()
			buffer.Append(testData)
			_ = buffer.UnsafeString()
		}
	})

	b.Run("MemoryReallocation_Test", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			entries := make([]*AccessLogEntry, 0, 1000)

			// Simulate growing the slice
			for j := 0; j < 5000; j++ {
				entry := &AccessLogEntry{IP: fmt.Sprintf("192.168.1.%d", j%255+1)}
				entries = append(entries, entry)
			}

			// Ensure entries is consumed so append result is used
			if len(entries) > 0 {
				_ = entries[len(entries)-1]
			}
		}
	})
}

// BenchmarkContextCheckFrequency tests the impact of context checking frequency
func BenchmarkContextCheckFrequency(b *testing.B) {
	logData := generateBenchmarkLogData(10000)

	frequencies := []struct {
		name string
		freq int
	}{
		{"Every_Line", 1},
		{"Every_10_Lines", 10},
		{"Every_50_Lines", 50},
		{"Every_100_Lines", 100},
		{"Every_500_Lines", 500},
	}

	for _, freq := range frequencies {
		b.Run(freq.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				reader := strings.NewReader(logData)
				ctx := context.Background()

				// Simulate context checking at different frequencies
				lineCount := 0
				for {
					line, err := reader.ReadByte()
					if err != nil {
						break
					}

					lineCount++
					if lineCount%freq.freq == 0 {
						select {
						case <-ctx.Done():
							b.Fatal("Context cancelled")
						default:
						}
					}

					// Simulate some work
					_ = line
				}
			}
		})
	}
}

// generateBenchmarkLogData generates realistic nginx log data for benchmarking
func generateBenchmarkLogData(lines int) string {
	var builder strings.Builder
	builder.Grow(lines * 200) // Pre-allocate space

	methods := []string{"GET", "POST", "PUT", "DELETE", "HEAD"}
	statuses := []string{"200", "404", "500", "301", "302"}
	paths := []string{"/", "/index.html", "/api/users", "/static/css/style.css", "/api/data"}
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
		"curl/7.68.0",
		"PostmanRuntime/7.29.0",
	}

	for i := 0; i < lines; i++ {
		ip := fmt.Sprintf("192.168.%d.%d", i%256, (i/256)%256)
		method := methods[i%len(methods)]
		path := paths[i%len(paths)]
		status := statuses[i%len(statuses)]
		size := 1000 + (i % 10000)
		userAgent := userAgents[i%len(userAgents)]

		line := fmt.Sprintf(`%s - - [25/Dec/2023:10:%02d:%02d +0000] "%s %s HTTP/1.1" %s %d "https://example.com" "%s"`,
			ip, (i/60)%24, i%60, method, path, status, size, userAgent)

		builder.WriteString(line)
		if i < lines-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// TestParseStreamCorrectness verifies that optimized implementations produce correct results
func TestParseStreamCorrectness(t *testing.T) {
	logData := `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0"
192.168.1.1 - - [25/Dec/2023:10:00:01 +0000] "POST /api/data HTTP/1.1" 201 567 "https://example.com" "curl/7.68.0"
10.0.0.1 - - [25/Dec/2023:10:00:02 +0000] "GET /style.css HTTP/1.1" 200 890 "https://example.com" "Mozilla/5.0"`

	config := DefaultParserConfig()
	config.MaxLineLength = 16 * 1024
	parser := NewParser(
		config,
		NewSimpleUserAgentParser(),
		&mockGeoIPService{},
	)

	ctx := context.Background()

	// Test original implementation
	originalResult, err := parser.ParseStream(ctx, strings.NewReader(logData))
	if err != nil {
		t.Fatalf("Original ParseStream failed: %v", err)
	}

	// Test stream implementations
	implementations := []struct {
		name string
		fn   func(context.Context, *strings.Reader) (*ParseResult, error)
	}{
		{
			"StreamParse",
			func(ctx context.Context, reader *strings.Reader) (*ParseResult, error) {
				return parser.StreamParse(ctx, reader)
			},
		},
		{
			"ChunkedParseStream",
			func(ctx context.Context, reader *strings.Reader) (*ParseResult, error) {
				return parser.ChunkedParseStream(ctx, reader, 1024)
			},
		},
		{
			"MemoryEfficientParseStream",
			func(ctx context.Context, reader *strings.Reader) (*ParseResult, error) {
				return parser.MemoryEfficientParseStream(ctx, reader)
			},
		},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			result, err := impl.fn(ctx, strings.NewReader(logData))
			if err != nil {
				t.Fatalf("%s failed: %v", impl.name, err)
			}

			// Compare basic metrics
			if result.Processed != originalResult.Processed {
				t.Errorf("%s processed count mismatch: got %d, want %d",
					impl.name, result.Processed, originalResult.Processed)
			}

			if result.Succeeded != originalResult.Succeeded {
				t.Errorf("%s succeeded count mismatch: got %d, want %d",
					impl.name, result.Succeeded, originalResult.Succeeded)
			}

			if len(result.Entries) != len(originalResult.Entries) {
				t.Errorf("%s entries count mismatch: got %d, want %d",
					impl.name, len(result.Entries), len(originalResult.Entries))
			}

			// Compare first entry details
			if len(result.Entries) > 0 && len(originalResult.Entries) > 0 {
				got := result.Entries[0]
				want := originalResult.Entries[0]

				if got.IP != want.IP {
					t.Errorf("%s first entry IP mismatch: got %s, want %s", impl.name, got.IP, want.IP)
				}

				if got.Status != want.Status {
					t.Errorf("%s first entry Status mismatch: got %d, want %d", impl.name, got.Status, want.Status)
				}
			}
		})
	}
}

// TestUnsafeStringConversion tests the safety of unsafe string conversion
func TestUnsafeStringConversion(t *testing.T) {
	testCases := [][]byte{
		[]byte("hello world"),
		[]byte(""),
		[]byte("127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] \"GET /index.html HTTP/1.1\" 200 1234"),
		[]byte("special chars: !@#$%^&*()"),
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("Case_%d", i), func(t *testing.T) {
			unsafeStr := unsafeBytesToString(testCase)
			safeStr := string(testCase)

			if unsafeStr != safeStr {
				t.Errorf("Unsafe conversion mismatch: unsafe=%s, safe=%s", unsafeStr, safeStr)
			}
		})
	}
}

// TestLineBufferOperations tests LineBuffer functionality
func TestLineBufferOperations(t *testing.T) {
	buffer := NewLineBuffer(1024)

	// Test basic operations
	testData := []byte("test data")
	buffer.Append(testData)

	if string(buffer.Bytes()) != "test data" {
		t.Errorf("Buffer content mismatch: got %s, want %s", buffer.Bytes(), "test data")
	}

	// Test reset
	buffer.Reset()
	if len(buffer.Bytes()) != 0 {
		t.Errorf("Buffer should be empty after reset, got length %d", len(buffer.Bytes()))
	}

	// Test growth
	largeData := make([]byte, 2048)
	for i := range largeData {
		largeData[i] = byte('a' + (i % 26))
	}

	buffer.Append(largeData)
	if len(buffer.Bytes()) != 2048 {
		t.Errorf("Buffer size mismatch: got %d, want 2048", len(buffer.Bytes()))
	}
}

// BenchmarkStreamBufferSizes tests different buffer sizes for streaming
func BenchmarkStreamBufferSizes(b *testing.B) {
	logData := generateBenchmarkLogData(10000)

	config := DefaultParserConfig()
	config.MaxLineLength = 16 * 1024
	parser := NewParser(
		config,
		NewSimpleUserAgentParser(),
		&mockGeoIPService{},
	)

	bufferSizes := []int{
		1024,  // 1KB
		4096,  // 4KB
		16384, // 16KB
		32768, // 32KB
		65536, // 64KB
	}

	for _, size := range bufferSizes {
		b.Run(fmt.Sprintf("BufferSize_%dKB", size/1024), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				reader := strings.NewReader(logData)
				ctx := context.Background()

				result, err := parser.ChunkedParseStream(ctx, reader, size)
				if err != nil {
					b.Fatal(err)
				}

				b.ReportMetric(float64(result.Processed), "lines")
				b.ReportMetric(float64(size), "buffer_bytes")
			}
		})
	}
}
