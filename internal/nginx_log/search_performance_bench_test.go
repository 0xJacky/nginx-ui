package nginx_log

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

// Benchmark configuration constants
const (
	BenchmarkLogEntriesSmall  = 10000    // 10K entries
	BenchmarkLogEntriesMedium = 100000   // 100K entries
	BenchmarkLogEntriesLarge  = 1000000  // 1M entries
	BenchmarkLogEntriesXLarge = 10000000 // 10M entries

	BenchmarkBatchSizeSmall  = 100
	BenchmarkBatchSizeMedium = 1000
	BenchmarkBatchSizeLarge  = 10000

	BenchmarkConcurrencyLow    = 1
	BenchmarkConcurrencyMedium = 4
	BenchmarkConcurrencyHigh   = 8
)

var (
	// Pre-generated test data for consistent benchmarking
	testIPs       []string
	testPaths     []string
	testMethods   []string
	testStatuses  []int
	testUserAgents []string
	benchmarkData []string
)

func init() {
	initBenchmarkTestData()
}

func initBenchmarkTestData() {
	// Initialize test data arrays for consistent benchmarking
	testIPs = []string{
		"192.168.1.1", "192.168.1.2", "10.0.0.1", "10.0.0.2", 
		"172.16.0.1", "172.16.0.2", "203.0.113.1", "203.0.113.2",
		"198.51.100.1", "198.51.100.2", "2001:db8::1", "2001:db8::2",
	}
	
	testPaths = []string{
		"/", "/api/v1/users", "/api/v1/posts", "/static/css/main.css",
		"/static/js/app.js", "/api/v1/auth/login", "/api/v1/auth/logout",
		"/api/v1/data", "/images/logo.png", "/favicon.ico", "/robots.txt",
		"/sitemap.xml", "/api/v1/search", "/admin/dashboard", "/user/profile",
	}
	
	testMethods = []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS"}
	
	testStatuses = []int{200, 201, 301, 302, 400, 401, 403, 404, 500, 502, 503}
	
	testUserAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (iPad; CPU OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Android 11; Mobile; rv:68.0) Gecko/68.0 Firefox/88.0",
		"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:88.0) Gecko/20100101 Firefox/88.0",
	}
}

func generateBenchmarkLogData(count int) []string {
	if len(benchmarkData) >= count {
		return benchmarkData[:count]
	}
	
	data := make([]string, count)
	baseTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	
	for i := 0; i < count; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Minute)
		ip := testIPs[rand.Intn(len(testIPs))]
		method := testMethods[rand.Intn(len(testMethods))]
		path := testPaths[rand.Intn(len(testPaths))]
		status := testStatuses[rand.Intn(len(testStatuses))]
		size := rand.Intn(10000) + 100
		userAgent := testUserAgents[rand.Intn(len(testUserAgents))]
		
		data[i] = fmt.Sprintf(
			`%s - - [%s] "%s %s HTTP/1.1" %d %d "-" "%s" %d.%03d %d.%03d`,
			ip,
			timestamp.Format("02/Jan/2006:15:04:05 -0700"),
			method,
			path,
			status,
			size,
			userAgent,
			rand.Intn(5), rand.Intn(1000),
			rand.Intn(2), rand.Intn(1000),
		)
	}
	
	// Cache the data for reuse
	if len(benchmarkData) == 0 {
		benchmarkData = data
	}
	
	return data
}

func setupBenchmarkIndexer(b *testing.B, entryCount int) (*LogIndexer, string, func()) {
	b.Helper()
	
	// Create temporary directory for benchmark index
	tempDir, err := os.MkdirTemp("", "nginx_search_bench")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Create test log file
	logFile := filepath.Join(tempDir, "benchmark.log")
	logData := generateBenchmarkLogData(entryCount)
	logContent := strings.Join(logData, "\n")
	
	err = os.WriteFile(logFile, []byte(logContent), 0644)
	if err != nil {
		b.Fatalf("Failed to write benchmark log file: %v", err)
	}
	
	// Create indexer
	indexPath := filepath.Join(tempDir, "index")
	index, err := createOrOpenIndex(indexPath)
	if err != nil {
		b.Fatalf("Failed to create index: %v", err)
	}
	
	uaParser := NewSimpleUserAgentParser()
	parser := NewOptimizedLogParser(uaParser)
	
	// Initialize cache with larger capacity for benchmarks
	cache, err := ristretto.NewCache(&ristretto.Config[string, *CachedSearchResult]{
		NumCounters: 1e8,     // 100M counters
		MaxCost:     1 << 30, // 1GB cache
		BufferItems: 64,
	})
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}
	
	indexer := &LogIndexer{
		index:      index,
		indexPath:  indexPath,
		parser:     parser,
		logPaths:   make(map[string]*LogFileInfo),
		indexBatch: BenchmarkBatchSizeLarge,
		cache:      cache,
	}
	
	// Parse and index the data directly (bypass safety checks for benchmarking)
	entries := make([]*AccessLogEntry, 0, entryCount)
	for _, line := range logData {
		if entry, err := parser.ParseLine(line); err == nil {
			entry.Raw = line
			entries = append(entries, entry)
		}
	}
	
	// Index entries directly
	batch := index.NewBatch()
	for i, entry := range entries {
		docID := fmt.Sprintf("doc_%d", i)
		doc := map[string]interface{}{
			"timestamp":    entry.Timestamp,
			"ip":           entry.IP,
			"method":       entry.Method,
			"path":         entry.Path,
			"protocol":     entry.Protocol,
			"status":       entry.Status,
			"bytes_sent":   entry.BytesSent,
			"request_time": entry.RequestTime,
			"referer":      entry.Referer,
			"user_agent":   entry.UserAgent,
			"browser":      entry.Browser,
			"browser_version": entry.BrowserVer,
			"os":           entry.OS,
			"os_version":   entry.OSVersion,
			"device_type":  entry.DeviceType,
			"raw":          entry.Raw,
		}
		
		if entry.UpstreamTime != nil {
			doc["upstream_time"] = *entry.UpstreamTime
		}
		
		err = batch.Index(docID, doc)
		if err != nil {
			b.Fatalf("Failed to add document to batch: %v", err)
		}
	}
	
	err = index.Batch(batch)
	if err != nil {
		b.Fatalf("Failed to execute batch: %v", err)
	}
	
	// Wait for indexing to complete
	time.Sleep(500 * time.Millisecond)
	
	cleanup := func() {
		indexer.Close()
		os.RemoveAll(tempDir)
	}
	
	return indexer, logFile, cleanup
}

// Benchmark basic search operations
func BenchmarkSearchLogs_Simple(b *testing.B) {
	sizes := []struct {
		name  string
		count int
	}{
		{"10K", BenchmarkLogEntriesSmall},
		{"100K", BenchmarkLogEntriesMedium},
		{"1M", BenchmarkLogEntriesLarge},
	}
	
	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			indexer, _, cleanup := setupBenchmarkIndexer(b, size.count)
			defer cleanup()
			
			req := &QueryRequest{
				Limit: 100,
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				_, err := indexer.SearchLogs(context.Background(), req)
				if err != nil {
					b.Fatalf("Search failed: %v", err)
				}
			}
		})
	}
}

// Benchmark IP-based searches
func BenchmarkSearchLogs_ByIP(b *testing.B) {
	indexer, _, cleanup := setupBenchmarkIndexer(b, BenchmarkLogEntriesMedium)
	defer cleanup()
	
	req := &QueryRequest{
		IP:    "192.168.1.1",
		Limit: 100,
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := indexer.SearchLogs(context.Background(), req)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// Benchmark method-based searches
func BenchmarkSearchLogs_ByMethod(b *testing.B) {
	indexer, _, cleanup := setupBenchmarkIndexer(b, BenchmarkLogEntriesMedium)
	defer cleanup()
	
	req := &QueryRequest{
		Method: "GET",
		Limit:  100,
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := indexer.SearchLogs(context.Background(), req)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// Benchmark status-based searches
func BenchmarkSearchLogs_ByStatus(b *testing.B) {
	indexer, _, cleanup := setupBenchmarkIndexer(b, BenchmarkLogEntriesMedium)
	defer cleanup()
	
	req := &QueryRequest{
		Status: []int{200, 404, 500},
		Limit:  100,
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := indexer.SearchLogs(context.Background(), req)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// Benchmark complex multi-field searches
func BenchmarkSearchLogs_Complex(b *testing.B) {
	indexer, _, cleanup := setupBenchmarkIndexer(b, BenchmarkLogEntriesMedium)
	defer cleanup()
	
	req := &QueryRequest{
		Method: "GET",
		Status: []int{200, 404},
		Path:   "/api",
		Limit:  100,
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := indexer.SearchLogs(context.Background(), req)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// Benchmark time range searches
func BenchmarkSearchLogs_TimeRange(b *testing.B) {
	indexer, _, cleanup := setupBenchmarkIndexer(b, BenchmarkLogEntriesMedium)
	defer cleanup()
	
	startTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	
	req := &QueryRequest{
		StartTime: startTime.Unix(),
		EndTime:   endTime.Unix(),
		Limit:     100,
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := indexer.SearchLogs(context.Background(), req)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// Benchmark pagination performance
func BenchmarkSearchLogs_Pagination(b *testing.B) {
	indexer, _, cleanup := setupBenchmarkIndexer(b, BenchmarkLogEntriesMedium)
	defer cleanup()
	
	pageSize := 50
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		offset := (i % 100) * pageSize // Simulate different pages
		req := &QueryRequest{
			Limit:  pageSize,
			Offset: offset,
		}
		
		_, err := indexer.SearchLogs(context.Background(), req)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// Benchmark sorting performance
func BenchmarkSearchLogs_Sorting(b *testing.B) {
	indexer, _, cleanup := setupBenchmarkIndexer(b, BenchmarkLogEntriesMedium)
	defer cleanup()
	
	sortFields := []string{"timestamp", "ip", "method", "status", "bytes_sent"}
	
	for _, field := range sortFields {
		b.Run(field, func(b *testing.B) {
			req := &QueryRequest{
				Limit:     100,
				SortBy:    field,
				SortOrder: "desc",
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				_, err := indexer.SearchLogs(context.Background(), req)
				if err != nil {
					b.Fatalf("Search failed: %v", err)
				}
			}
		})
	}
}

// Benchmark cache performance
func BenchmarkSearchLogs_Cache(b *testing.B) {
	indexer, _, cleanup := setupBenchmarkIndexer(b, BenchmarkLogEntriesMedium)
	defer cleanup()
	
	req := &QueryRequest{
		IP:    "192.168.1.1",
		Limit: 100,
	}
	
	// Prime the cache
	_, err := indexer.SearchLogs(context.Background(), req)
	if err != nil {
		b.Fatalf("Failed to prime cache: %v", err)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := indexer.SearchLogs(context.Background(), req)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// Benchmark concurrent search performance
func BenchmarkSearchLogs_Concurrent(b *testing.B) {
	concurrencies := []int{
		BenchmarkConcurrencyLow,
		BenchmarkConcurrencyMedium,
		BenchmarkConcurrencyHigh,
		runtime.NumCPU(),
	}
	
	for _, concurrency := range concurrencies {
		b.Run(fmt.Sprintf("Workers%d", concurrency), func(b *testing.B) {
			indexer, _, cleanup := setupBenchmarkIndexer(b, BenchmarkLogEntriesMedium)
			defer cleanup()
			
			// Create different search requests for each worker
			requests := make([]*QueryRequest, concurrency)
			for i := 0; i < concurrency; i++ {
				requests[i] = &QueryRequest{
					IP:    testIPs[i%len(testIPs)],
					Limit: 100,
				}
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			b.RunParallel(func(pb *testing.PB) {
				workerID := 0
				for pb.Next() {
					req := requests[workerID%concurrency]
					_, err := indexer.SearchLogs(context.Background(), req)
					if err != nil {
						b.Fatalf("Search failed: %v", err)
					}
					workerID++
				}
			})
		})
	}
}

// Benchmark large result set handling
func BenchmarkSearchLogs_LargeResults(b *testing.B) {
	indexer, _, cleanup := setupBenchmarkIndexer(b, BenchmarkLogEntriesLarge)
	defer cleanup()
	
	resultSizes := []int{100, 1000, 10000}
	
	for _, size := range resultSizes {
		b.Run(fmt.Sprintf("Results%d", size), func(b *testing.B) {
			req := &QueryRequest{
				Limit: size,
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				_, err := indexer.SearchLogs(context.Background(), req)
				if err != nil {
					b.Fatalf("Search failed: %v", err)
				}
			}
		})
	}
}

// Benchmark text search performance
func BenchmarkSearchLogs_TextSearch(b *testing.B) {
	indexer, _, cleanup := setupBenchmarkIndexer(b, BenchmarkLogEntriesMedium)
	defer cleanup()
	
	queries := []string{
		"api",
		"GET",
		"200",
		"Mozilla",
		"/static",
	}
	
	for _, query := range queries {
		b.Run(query, func(b *testing.B) {
			req := &QueryRequest{
				Query: query,
				Limit: 100,
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				_, err := indexer.SearchLogs(context.Background(), req)
				if err != nil {
					b.Fatalf("Search failed: %v", err)
				}
			}
		})
	}
}

// Benchmark memory usage during search
func BenchmarkSearchLogs_Memory(b *testing.B) {
	indexer, _, cleanup := setupBenchmarkIndexer(b, BenchmarkLogEntriesLarge)
	defer cleanup()
	
	req := &QueryRequest{
		Limit: 1000,
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	
	for i := 0; i < b.N; i++ {
		_, err := indexer.SearchLogs(context.Background(), req)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
	
	runtime.GC()
	runtime.ReadMemStats(&m2)
	
	b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/search")
}

// Comprehensive performance comparison benchmark
func BenchmarkSearchLogs_Comprehensive(b *testing.B) {
	// Test different data sizes with various search patterns
	scenarios := []struct {
		name      string
		dataSize  int
		req       *QueryRequest
	}{
		{
			name:     "Small_Simple",
			dataSize: BenchmarkLogEntriesSmall,
			req:      &QueryRequest{Limit: 100},
		},
		{
			name:     "Medium_IP",
			dataSize: BenchmarkLogEntriesMedium,
			req:      &QueryRequest{IP: "192.168.1.1", Limit: 100},
		},
		{
			name:     "Large_Complex",
			dataSize: BenchmarkLogEntriesLarge,
			req:      &QueryRequest{Method: "GET", Status: []int{200}, Limit: 100},
		},
	}
	
	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			indexer, _, cleanup := setupBenchmarkIndexer(b, scenario.dataSize)
			defer cleanup()
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				result, err := indexer.SearchLogs(context.Background(), scenario.req)
				if err != nil {
					b.Fatalf("Search failed: %v", err)
				}
				
				// Report additional metrics
				if i == 0 {
					b.ReportMetric(float64(result.Total), "total_results")
					b.ReportMetric(float64(len(result.Entries)), "returned_results")
					b.ReportMetric(float64(result.Took*1000000), "search_time_ns")
				}
			}
		})
	}
}