package nginx_log

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/simple"
	"github.com/blevesearch/bleve/v2/search/query"
)

// OptimizedBenchmarkConfig for better performance
type OptimizedBenchmarkConfig struct {
	TotalDocuments int
	BatchSize      int
	NumWorkers     int
	IndexPath      string
}

// BenchmarkOptimizedIndexing tests optimized indexing performance
func BenchmarkOptimizedIndexing(b *testing.B) {
	config := OptimizedBenchmarkConfig{
		TotalDocuments: 1000000,
		BatchSize:      50000,    // Larger batches for better throughput
		NumWorkers:     16,       // More workers
		IndexPath:      filepath.Join(os.TempDir(), "bench_optimized"),
	}

	// Cleanup
	defer os.RemoveAll(config.IndexPath)

	b.Run("IndexAndSearch", func(b *testing.B) {
		// Create optimized index
		index, err := createHighPerformanceIndex(config.IndexPath)
		if err != nil {
			b.Fatalf("Failed to create index: %v", err)
		}
		defer index.Close()

		// Phase 1: Fast bulk indexing
		b.Run("BulkIndexing", func(b *testing.B) {
			start := time.Now()
			count := fastBulkIndex(b, index, config)
			duration := time.Since(start)
			
			docsPerSec := float64(count) / duration.Seconds()
			b.ReportMetric(float64(count), "docs")
			b.ReportMetric(docsPerSec, "docs/sec")
			b.Logf("Indexed %d docs in %v (%.0f docs/sec)", count, duration, docsPerSec)
		})

		// Phase 2: Optimized search with pagination
		b.Run("PaginatedSearch", func(b *testing.B) {
			benchmarkOptimizedSearch(b, index)
		})
	})
}

// createHighPerformanceIndex creates an index optimized for speed
func createHighPerformanceIndex(path string) (bleve.Index, error) {
	// Create minimal mapping - only index what we search
	indexMapping := bleve.NewIndexMapping()
	
	// Disable default mapping
	indexMapping.DefaultMapping.Enabled = false
	indexMapping.TypeField = "_type"
	indexMapping.DefaultAnalyzer = keyword.Name
	
	// Document mapping with minimal fields
	docMapping := bleve.NewDocumentMapping()
	docMapping.Enabled = true
	docMapping.Dynamic = false
	
	// Only index fields we actually search on
	// Method - keyword only
	methodField := bleve.NewTextFieldMapping()
	methodField.Analyzer = keyword.Name
	methodField.Store = false
	methodField.IncludeInAll = false
	docMapping.AddFieldMappingsAt("method", methodField)
	
	// Status - numeric for range queries
	statusField := bleve.NewNumericFieldMapping()
	statusField.Store = false
	statusField.IncludeInAll = false
	statusField.Index = true
	docMapping.AddFieldMappingsAt("status", statusField)
	
	// Path - simple analyzer (faster than standard)
	pathField := bleve.NewTextFieldMapping()
	pathField.Analyzer = simple.Name
	pathField.Store = false
	pathField.IncludeInAll = false
	docMapping.AddFieldMappingsAt("path", pathField)
	
	// Message - simple analyzer for basic text search
	messageField := bleve.NewTextFieldMapping()
	messageField.Analyzer = simple.Name
	messageField.Store = false
	messageField.IncludeInAll = false
	docMapping.AddFieldMappingsAt("message", messageField)
	
	indexMapping.AddDocumentMapping("doc", docMapping)
	indexMapping.DefaultMapping = docMapping
	
	// Create index
	return bleve.New(path, indexMapping)
}

// fastBulkIndex performs optimized bulk indexing
func fastBulkIndex(b *testing.B, index bleve.Index, config OptimizedBenchmarkConfig) int {
	var indexed int64
	var wg sync.WaitGroup
	
	// Pre-generate data to avoid generation overhead during indexing
	b.Logf("Pre-generating %d documents...", config.TotalDocuments)
	allDocs := pregenerateDocuments(config.TotalDocuments)
	
	// Create batches
	batches := make([][]map[string]interface{}, 0)
	for i := 0; i < len(allDocs); i += config.BatchSize {
		end := i + config.BatchSize
		if end > len(allDocs) {
			end = len(allDocs)
		}
		batches = append(batches, allDocs[i:end])
	}
	
	b.Logf("Starting bulk indexing with %d batches of %d docs", len(batches), config.BatchSize)
	
	// Process batches with worker pool
	batchChan := make(chan []map[string]interface{}, len(batches))
	for _, batch := range batches {
		batchChan <- batch
	}
	close(batchChan)
	
	// Start workers
	for w := 0; w < config.NumWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for docs := range batchChan {
				batch := index.NewBatch()
				
				for _, doc := range docs {
					batch.Index(doc["id"].(string), doc)
				}
				
				// Execute batch
				if err := index.Batch(batch); err != nil {
					b.Logf("Worker %d batch failed: %v", workerID, err)
				} else {
					atomic.AddInt64(&indexed, int64(len(docs)))
					if current := atomic.LoadInt64(&indexed); current%100000 == 0 {
						b.Logf("Progress: %d docs indexed", current)
					}
				}
			}
		}(w)
	}
	
	wg.Wait()
	return int(indexed)
}

// pregenerateDocuments generates all documents upfront
func pregenerateDocuments(count int) []map[string]interface{} {
	docs := make([]map[string]interface{}, count)
	
	methods := []string{"GET", "POST", "PUT", "DELETE"}
	paths := []string{"/api/users", "/api/products", "/api/orders", "/api/search"}
	messages := []string{"success", "error", "timeout", "processed"}
	
	for i := 0; i < count; i++ {
		docs[i] = map[string]interface{}{
			"id":      fmt.Sprintf("doc_%d", i),
			"method":  methods[i%len(methods)],
			"status":  200 + (i%5)*100, // 200, 300, 400, 500, 600
			"path":    paths[i%len(paths)],
			"message": messages[i%len(messages)],
		}
	}
	
	return docs
}

// benchmarkOptimizedSearch tests search with pagination
func benchmarkOptimizedSearch(b *testing.B, index bleve.Index) {
	// Wait for index to settle
	time.Sleep(100 * time.Millisecond)
	
	testCases := []struct {
		name    string
		query   query.Query
		size    int
	}{
		{
			name: "SimpleTermQuery",
			query: func() query.Query {
				q := bleve.NewTermQuery("GET")
				q.SetField("method")
				return q
			}(),
			size: 10, // Only get top 10 results
		},
		{
			name: "NumericRange_Paginated",
			query: func() query.Query {
				min := float64(200)
				max := float64(299)
				q := bleve.NewNumericRangeQuery(&min, &max)
				q.SetField("status")
				return q
			}(),
			size: 10,
		},
		{
			name: "TextSearch_Paginated",
			query: func() query.Query {
				q := bleve.NewMatchQuery("success")
				q.SetField("message")
				return q
			}(),
			size: 10,
		},
	}
	
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			
			var totalLatency int64
			var totalHits uint64
			
			for i := 0; i < b.N; i++ {
				req := bleve.NewSearchRequest(tc.query)
				req.Size = tc.size
				req.From = 0
				// Disable expensive features
				req.Explain = false
				req.IncludeLocations = false
				
				start := time.Now()
				result, err := index.Search(req)
				latency := time.Since(start)
				
				if err != nil {
					b.Errorf("Search failed: %v", err)
					continue
				}
				
				totalLatency += latency.Microseconds()
				totalHits += result.Total
			}
			
			avgLatency := totalLatency / int64(b.N)
			b.ReportMetric(float64(avgLatency), "μs/op")
			b.ReportMetric(float64(totalHits)/float64(b.N), "total_hits")
		})
	}
}

// BenchmarkConcurrentOptimizedSearch tests concurrent search performance
func BenchmarkConcurrentOptimizedSearch(b *testing.B) {
	config := OptimizedBenchmarkConfig{
		TotalDocuments: 100000, // Smaller dataset for quick concurrent test
		BatchSize:      10000,
		NumWorkers:     8,
		IndexPath:      filepath.Join(os.TempDir(), "bench_concurrent"),
	}
	
	// Cleanup
	defer os.RemoveAll(config.IndexPath)
	
	// Create and populate index
	index, err := createHighPerformanceIndex(config.IndexPath)
	if err != nil {
		b.Fatalf("Failed to create index: %v", err)
	}
	defer index.Close()
	
	// Quick index
	fastBulkIndex(b, index, config)
	
	// Test different concurrency levels
	concurrencyLevels := []int{1, 10, 50, 100}
	
	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrent_%d", concurrency), func(b *testing.B) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			var totalQueries int64
			var totalLatency int64
			var wg sync.WaitGroup
			
			// Simple query for consistent testing
			q := bleve.NewTermQuery("GET")
			q.SetField("method")
			searchQuery := q
			
			start := time.Now()
			
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					
					for {
						select {
						case <-ctx.Done():
							return
						default:
							req := bleve.NewSearchRequest(searchQuery)
							req.Size = 10
							req.From = rand.Intn(100) * 10 // Random pagination
							
							queryStart := time.Now()
							_, err := index.Search(req)
							queryLatency := time.Since(queryStart)
							
							if err == nil {
								atomic.AddInt64(&totalQueries, 1)
								atomic.AddInt64(&totalLatency, queryLatency.Microseconds())
							}
							
							if atomic.LoadInt64(&totalQueries) >= 1000 {
								cancel()
								return
							}
						}
					}
				}()
			}
			
			wg.Wait()
			duration := time.Since(start)
			
			queries := atomic.LoadInt64(&totalQueries)
			qps := float64(queries) / duration.Seconds()
			avgLatency := float64(totalLatency) / float64(queries)
			
			b.ReportMetric(qps, "qps")
			b.ReportMetric(avgLatency, "μs/query")
			b.Logf("Concurrency %d: %.0f qps, %.0fμs avg latency", concurrency, qps, avgLatency)
		})
	}
}