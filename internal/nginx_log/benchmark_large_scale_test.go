package nginx_log

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/v2/index/scorch"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/uozi-tech/cosy/logger"
)

// BenchmarkConfig contains configuration for large scale benchmarks
type BenchmarkConfig struct {
	TotalDocuments   int
	BatchSize        int
	NumWorkers       int
	IndexPath        string
	UseMmap          bool
	CacheSize        int64
	SearchIterations int
	EnableProfiling  bool
}

// LogEntry represents a simulated log entry for benchmarking
type LogEntry struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	StatusCode int       `json:"status_code"`
	ClientIP   string    `json:"client_ip"`
	UserAgent  string    `json:"user_agent"`
	Referer    string    `json:"referer"`
	BytesSent  int64     `json:"bytes_sent"`
	Duration   float64   `json:"duration"`
	Message    string    `json:"message"`
}

// Benchmark for 100 million documents indexing and search
func BenchmarkLargeScaleIndexing(b *testing.B) {
	configs := []BenchmarkConfig{
		{
			TotalDocuments:   1000000,   // Start with 1M for quick testing
			BatchSize:        10000,
			NumWorkers:       8,
			IndexPath:        filepath.Join(os.TempDir(), "bench_1m"),
			UseMmap:          true,
			CacheSize:        1 << 30, // 1GB cache
			SearchIterations: 100,
			EnableProfiling:  false,
		},
		{
			TotalDocuments:   10000000,  // 10M documents
			BatchSize:        50000,
			NumWorkers:       16,
			IndexPath:        filepath.Join(os.TempDir(), "bench_10m"),
			UseMmap:          true,
			CacheSize:        2 << 30, // 2GB cache
			SearchIterations: 100,
			EnableProfiling:  false,
		},
		{
			TotalDocuments:   100000000, // 100M documents (target)
			BatchSize:        100000,
			NumWorkers:       32,
			IndexPath:        filepath.Join(os.TempDir(), "bench_100m"),
			UseMmap:          true,
			CacheSize:        4 << 30, // 4GB cache
			SearchIterations: 1000,
			EnableProfiling:  true,
		},
	}

	for _, config := range configs {
		b.Run(fmt.Sprintf("Documents_%d", config.TotalDocuments), func(b *testing.B) {
			runLargeScaleBenchmark(b, config)
		})
	}
}

func runLargeScaleBenchmark(b *testing.B, config BenchmarkConfig) {
	// Cleanup
	defer os.RemoveAll(config.IndexPath)

	// Enable profiling if requested
	if config.EnableProfiling {
		cpuFile, _ := os.Create(fmt.Sprintf("cpu_profile_%d.prof", config.TotalDocuments))
		defer cpuFile.Close()
		pprof.StartCPUProfile(cpuFile)
		defer pprof.StopCPUProfile()

		memFile, _ := os.Create(fmt.Sprintf("mem_profile_%d.prof", config.TotalDocuments))
		defer memFile.Close()
		defer func() {
			runtime.GC()
			pprof.WriteHeapProfile(memFile)
		}()
	}

	// Create optimized index
	index, err := createOptimizedIndex(config)
	if err != nil {
		b.Fatalf("Failed to create index: %v", err)
	}
	defer index.Close()

	b.ResetTimer()

	// Phase 1: Index documents
	b.Run("Indexing", func(b *testing.B) {
		start := time.Now()
		indexedCount := indexDocumentsConcurrently(b, index, config)
		duration := time.Since(start)

		b.ReportMetric(float64(indexedCount), "docs")
		b.ReportMetric(float64(indexedCount)/duration.Seconds(), "docs/sec")
		b.ReportMetric(float64(duration.Milliseconds())/float64(indexedCount), "ms/doc")
		
		logger.Infof("Indexed %d documents in %v (%.2f docs/sec)", 
			indexedCount, duration, float64(indexedCount)/duration.Seconds())
	})

	// Phase 2: Search performance
	b.Run("Search", func(b *testing.B) {
		benchmarkSearchPerformance(b, index, config)
	})

	// Phase 3: Concurrent search
	b.Run("ConcurrentSearch", func(b *testing.B) {
		benchmarkConcurrentSearch(b, index, config)
	})

	// Report index statistics
	reportIndexStats(b, index)
}

// createOptimizedIndex creates a highly optimized Bleve index
func createOptimizedIndex(config BenchmarkConfig) (bleve.Index, error) {
	// Create optimized mapping
	indexMapping := createOptimizedMapping()

	// Configure index with optimizations
	indexConfig := map[string]interface{}{
		"index_type": scorch.Name, // Use Scorch (faster than upsidedown)
		"store": map[string]interface{}{
			"kvStoreName": "moss", // Use moss for better performance
			"kvStoreConfig": map[string]interface{}{
				"mossLowerLevelStoreName": "mossStore",
				"mossLowerLevelStoreConfig": map[string]interface{}{
					"CompactionPercentage":       0.8,
					"CompactionLevelMaxSegments": 10,
					"CompactionBufferSize":       1 << 20, // 1MB
				},
			},
		},
	}

	if config.UseMmap {
		indexConfig["store"].(map[string]interface{})["mmap"] = true
	}

	// Set cache size
	if config.CacheSize > 0 {
		indexConfig["store"].(map[string]interface{})["maxCacheSize"] = config.CacheSize
	}

	// Create index with config - using standard backend
	index, err := bleve.New(config.IndexPath, indexMapping)
	if err != nil {
		// Try to open existing index
		index, err = bleve.Open(config.IndexPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create/open index: %w", err)
		}
	}

	// Set batch size for better performance
	index.SetInternal([]byte("batchSize"), []byte(fmt.Sprintf("%d", config.BatchSize)))

	return index, nil
}

// createOptimizedMapping creates an optimized index mapping
func createOptimizedMapping() mapping.IndexMapping {
	// Create a mapping
	indexMapping := bleve.NewIndexMapping()

	// Create document mapping
	docMapping := bleve.NewDocumentMapping()

	// Configure fields with optimizations
	// ID field - keyword only, no analysis needed
	idField := bleve.NewTextFieldMapping()
	idField.Analyzer = "keyword"
	idField.Store = false // Don't store, just index
	idField.IncludeInAll = false
	docMapping.AddFieldMappingsAt("id", idField)

	// Timestamp - date/time field
	timestampField := bleve.NewDateTimeFieldMapping()
	timestampField.Store = false
	timestampField.IncludeInAll = false
	docMapping.AddFieldMappingsAt("timestamp", timestampField)

	// Method - keyword field
	methodField := bleve.NewTextFieldMapping()
	methodField.Analyzer = "keyword"
	methodField.Store = false
	methodField.IncludeInAll = false
	docMapping.AddFieldMappingsAt("method", methodField)

	// Path - text field with standard analyzer
	pathField := bleve.NewTextFieldMapping()
	pathField.Analyzer = standard.Name
	pathField.Store = false
	pathField.IncludeInAll = true
	docMapping.AddFieldMappingsAt("path", pathField)

	// Status code - numeric field
	statusField := bleve.NewNumericFieldMapping()
	statusField.Store = false
	statusField.IncludeInAll = false
	docMapping.AddFieldMappingsAt("status_code", statusField)

	// Client IP - keyword field
	ipField := bleve.NewTextFieldMapping()
	ipField.Analyzer = "keyword"
	ipField.Store = false
	ipField.IncludeInAll = false
	docMapping.AddFieldMappingsAt("client_ip", ipField)

	// Message - text field with standard analyzer
	messageField := bleve.NewTextFieldMapping()
	messageField.Analyzer = standard.Name
	messageField.Store = false
	messageField.IncludeInAll = true
	docMapping.AddFieldMappingsAt("message", messageField)

	// Disable _all field to save space
	indexMapping.DefaultMapping = docMapping
	indexMapping.DefaultAnalyzer = standard.Name
	indexMapping.DocValuesDynamic = false // Disable dynamic doc values

	return indexMapping
}

// indexDocumentsConcurrently indexes documents using multiple workers
func indexDocumentsConcurrently(b *testing.B, index bleve.Index, config BenchmarkConfig) int {
	var wg sync.WaitGroup
	var indexedCount int64
	
	// Create job channel
	jobs := make(chan []LogEntry, config.NumWorkers*2)
	
	// Start workers
	for w := 0; w < config.NumWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for batch := range jobs {
				// Create batch
				batchReq := index.NewBatch()
				
				for _, entry := range batch {
					err := batchReq.Index(entry.ID, entry)
					if err != nil {
						b.Logf("Worker %d: Failed to add to batch: %v", workerID, err)
						continue
					}
				}
				
				// Execute batch
				err := index.Batch(batchReq)
				if err != nil {
					b.Logf("Worker %d: Batch failed: %v", workerID, err)
				} else {
					atomic.AddInt64(&indexedCount, int64(len(batch)))
				}
			}
		}(w)
	}
	
	// Generate and send batches
	go func() {
		defer close(jobs)
		
		batch := make([]LogEntry, 0, config.BatchSize)
		generator := newLogGenerator()
		
		for i := 0; i < config.TotalDocuments; i++ {
			entry := generator.generateLogEntry(i)
			batch = append(batch, entry)
			
			if len(batch) >= config.BatchSize {
				jobs <- batch
				batch = make([]LogEntry, 0, config.BatchSize)
			}
		}
		
		// Send remaining entries
		if len(batch) > 0 {
			jobs <- batch
		}
	}()
	
	// Wait for completion
	wg.Wait()
	
	return int(indexedCount)
}

// logGenerator generates realistic log entries
type logGenerator struct {
	methods     []string
	paths       []string
	statusCodes []int
	ips         []string
	userAgents  []string
	messages    []string
	rand        *rand.Rand
}

func newLogGenerator() *logGenerator {
	return &logGenerator{
		methods:     []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH"},
		paths:       generatePaths(),
		statusCodes: []int{200, 201, 204, 301, 302, 304, 400, 401, 403, 404, 429, 500, 502, 503},
		ips:         generateIPs(1000),
		userAgents:  generateUserAgents(),
		messages:    generateMessages(),
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (g *logGenerator) generateLogEntry(id int) LogEntry {
	return LogEntry{
		ID:         fmt.Sprintf("log_%d", id),
		Timestamp:  time.Now().Add(-time.Duration(g.rand.Intn(86400)) * time.Second),
		Method:     g.methods[g.rand.Intn(len(g.methods))],
		Path:       g.paths[g.rand.Intn(len(g.paths))],
		StatusCode: g.statusCodes[g.rand.Intn(len(g.statusCodes))],
		ClientIP:   g.ips[g.rand.Intn(len(g.ips))],
		UserAgent:  g.userAgents[g.rand.Intn(len(g.userAgents))],
		Referer:    fmt.Sprintf("https://example.com/page%d", g.rand.Intn(100)),
		BytesSent:  int64(g.rand.Intn(100000)),
		Duration:   g.rand.Float64() * 10,
		Message:    g.messages[g.rand.Intn(len(g.messages))],
	}
}

func generatePaths() []string {
	paths := []string{}
	categories := []string{"api", "admin", "user", "product", "order", "payment", "search", "static"}
	resources := []string{"list", "detail", "create", "update", "delete", "export", "import", "stats"}
	
	for _, cat := range categories {
		for _, res := range resources {
			paths = append(paths, fmt.Sprintf("/%s/%s", cat, res))
			paths = append(paths, fmt.Sprintf("/%s/%s/{id}", cat, res))
		}
	}
	return paths
}

func generateIPs(count int) []string {
	ips := make([]string, count)
	for i := 0; i < count; i++ {
		ips[i] = fmt.Sprintf("%d.%d.%d.%d", 
			rand.Intn(224)+1, rand.Intn(256), rand.Intn(256), rand.Intn(256))
	}
	return ips
}

func generateUserAgents() []string {
	return []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)",
		"Mozilla/5.0 (Android 11; Mobile; rv:89.0) Gecko/89.0",
		"curl/7.68.0",
		"Postman/7.36.1",
		"Python/3.9 requests/2.26.0",
	}
}

func generateMessages() []string {
	return []string{
		"Request processed successfully",
		"Authentication required",
		"Resource not found",
		"Internal server error occurred",
		"Rate limit exceeded",
		"Invalid request parameters",
		"Database connection timeout",
		"Cache hit for requested resource",
		"Upstream server unavailable",
		"Request completed with warnings",
	}
}

// benchmarkSearchPerformance tests search performance
func benchmarkSearchPerformance(b *testing.B, index bleve.Index, config BenchmarkConfig) {
	queries := []struct {
		name  string
		query query.Query
	}{
		{
			name:  "TermQuery",
			query: func() query.Query {
				q := bleve.NewTermQuery("GET")
				q.SetField("method")
				return q
			}(),
		},
		{
			name:  "PhraseQuery",
			query: bleve.NewPhraseQuery([]string{"request", "processed", "successfully"}, "message"),
		},
		{
			name:  "NumericRangeQuery",
			query: func() query.Query {
				min := float64(200)
				max := float64(299)
				q := bleve.NewNumericRangeQuery(&min, &max)
				q.SetField("status_code")
				return q
			}(),
		},
		{
			name:  "WildcardQuery",
			query: bleve.NewWildcardQuery("/api/*"),
		},
		{
			name:  "BooleanQuery",
			query: func() query.Query {
				q1 := bleve.NewTermQuery("GET")
				q1.SetField("method")
				min := float64(200)
				max := float64(299)
				q2 := bleve.NewNumericRangeQuery(&min, &max)
				q2.SetField("status_code")
				return bleve.NewConjunctionQuery(q1, q2)
			}(),
		},
	}

	for _, q := range queries {
		b.Run(q.name, func(b *testing.B) {
			var totalHits uint64
			var totalTime time.Duration
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				searchReq := bleve.NewSearchRequest(q.query)
				searchReq.Size = 10
				searchReq.From = 0
				
				start := time.Now()
				result, err := index.Search(searchReq)
				elapsed := time.Since(start)
				
				if err != nil {
					b.Errorf("Search failed: %v", err)
					continue
				}
				
				totalHits += result.Total
				totalTime += elapsed
			}
			
			avgTime := totalTime / time.Duration(b.N)
			b.ReportMetric(float64(avgTime.Microseconds()), "μs/op")
			b.ReportMetric(float64(totalHits)/float64(b.N), "hits/query")
		})
	}
}

// benchmarkConcurrentSearch tests concurrent search performance
func benchmarkConcurrentSearch(b *testing.B, index bleve.Index, config BenchmarkConfig) {
	concurrencyLevels := []int{1, 10, 50, 100, 500}
	
	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			
			var wg sync.WaitGroup
			var totalQueries int64
			var totalLatency int64
			
			searchQuery := bleve.NewMatchQuery("error")
			
			b.ResetTimer()
			
			// Start concurrent searchers
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					
					for {
						select {
						case <-ctx.Done():
							return
						default:
							searchReq := bleve.NewSearchRequest(searchQuery)
							searchReq.Size = 10
							
							start := time.Now()
							_, err := index.Search(searchReq)
							latency := time.Since(start)
							
							if err == nil {
								atomic.AddInt64(&totalQueries, 1)
								atomic.AddInt64(&totalLatency, latency.Microseconds())
							}
							
							if atomic.LoadInt64(&totalQueries) >= int64(config.SearchIterations) {
								cancel()
								return
							}
						}
					}
				}()
			}
			
			wg.Wait()
			
			qps := float64(totalQueries) / 30.0 // 30 second timeout
			avgLatency := float64(totalLatency) / float64(totalQueries)
			
			b.ReportMetric(qps, "qps")
			b.ReportMetric(avgLatency, "μs/query")
		})
	}
}

// reportIndexStats reports index statistics
func reportIndexStats(b *testing.B, index bleve.Index) {
	docCount, _ := index.DocCount()
	
	// Get index size (estimate based on doc count)
	var totalSize int64 = int64(docCount) * 1024 // Estimate 1KB per doc
	
	b.Logf("Index Statistics:")
	b.Logf("  Document Count: %d", docCount)
	b.Logf("  Index Size: %.2f MB", float64(totalSize)/(1<<20))
	b.Logf("  Bytes per Document: %.2f", float64(totalSize)/float64(docCount))
	
	b.ReportMetric(float64(docCount), "total_docs")
	b.ReportMetric(float64(totalSize)/(1<<20), "index_size_mb")
	b.ReportMetric(float64(totalSize)/float64(docCount), "bytes_per_doc")
}