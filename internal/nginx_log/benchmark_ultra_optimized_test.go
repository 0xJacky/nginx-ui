package nginx_log

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/index/scorch"
	"github.com/blevesearch/bleve/v2/search/query"
)

// BenchmarkUltraOptimized tests ultra-optimized Bleve performance based on official docs
func BenchmarkUltraOptimized(b *testing.B) {
	config := struct {
		TotalDocs  int
		BatchSize  int
		NumWorkers int
		IndexPath  string
	}{
		TotalDocs:  1000000,
		BatchSize:  100000, // Large batches as per Bleve docs
		NumWorkers: runtime.NumCPU() * 2,
		IndexPath:  filepath.Join(os.TempDir(), "bench_ultra"),
	}

	defer os.RemoveAll(config.IndexPath)

	// Create ultra-optimized index based on Bleve performance docs
	index, err := createUltraOptimizedIndex(config.IndexPath)
	if err != nil {
		b.Fatalf("Failed to create index: %v", err)
	}
	defer index.Close()

	b.Run("UltraFastIndexing", func(b *testing.B) {
		start := time.Now()
		count := ultraFastBulkIndex(b, index, config.TotalDocs, config.BatchSize, config.NumWorkers)
		duration := time.Since(start)
		
		docsPerSec := float64(count) / duration.Seconds()
		b.ReportMetric(float64(count), "docs")
		b.ReportMetric(docsPerSec, "docs/sec")
		b.Logf("Indexed %d docs in %v (%.0f docs/sec)", count, duration, docsPerSec)
	})

	// Wait for index to settle
	time.Sleep(500 * time.Millisecond)

	b.Run("OptimizedSearch", func(b *testing.B) {
		benchmarkUltraOptimizedSearch(b, index)
	})
}

// createUltraOptimizedIndex creates index with all performance optimizations from Bleve docs
func createUltraOptimizedIndex(path string) (bleve.Index, error) {
	// Minimal mapping - based on Bleve performance recommendations
	indexMapping := bleve.NewIndexMapping()
	
	// Disable unnecessary features for performance
	indexMapping.DefaultMapping.Enabled = false
	indexMapping.TypeField = ""  // Disable type field
	indexMapping.DefaultAnalyzer = keyword.Name
	indexMapping.DocValuesDynamic = false  // Disable dynamic doc values
	
	// Create minimal document mapping
	docMapping := bleve.NewDocumentMapping()
	docMapping.Enabled = true
	docMapping.Dynamic = false  // No dynamic fields
	
	// Only index essential fields with optimizations
	
	// Method field - keyword only, no storage
	methodField := bleve.NewTextFieldMapping()
	methodField.Analyzer = keyword.Name
	methodField.Store = false
	methodField.IncludeInAll = false
	methodField.IncludeTermVectors = false  // Disable term vectors
	methodField.DocValues = true  // Enable for sorting/faceting
	docMapping.AddFieldMappingsAt("method", methodField)
	
	// Status field - numeric with doc values
	statusField := bleve.NewNumericFieldMapping()
	statusField.Store = false
	statusField.IncludeInAll = false
	statusField.DocValues = true  // Enable for range queries
	docMapping.AddFieldMappingsAt("status", statusField)
	
	// Path field - keyword for exact match
	pathField := bleve.NewTextFieldMapping()
	pathField.Analyzer = keyword.Name
	pathField.Store = false
	pathField.IncludeInAll = false
	pathField.IncludeTermVectors = false
	pathField.DocValues = false  // Not needed for search
	docMapping.AddFieldMappingsAt("path", pathField)
	
	indexMapping.AddDocumentMapping("doc", docMapping)
	indexMapping.DefaultMapping = docMapping
	
	// Advanced index configuration based on Bleve persister docs
	kvConfig := map[string]interface{}{
		"index_type": scorch.Name,  // Use Scorch backend
		"scorchPersisterOptions": map[string]interface{}{
			"NumPersisterWorkers":           4,  // Parallel persistence
			"MaxSizeInMemoryMergePerWorker": 100 * 1024 * 1024,  // 100MB per worker
		},
		"scorchMergePlanOptions": map[string]interface{}{
			"FloorSegmentFileSize": 20 * 1024 * 1024,  // 20MB segments
		},
	}
	
	// Create index with optimized config
	return bleve.NewUsing(path, indexMapping, scorch.Name, scorch.Name, kvConfig)
}

// ultraFastBulkIndex performs ultra-fast bulk indexing
func ultraFastBulkIndex(b *testing.B, index bleve.Index, totalDocs, batchSize, numWorkers int) int {
	var indexed int64
	
	// Pre-generate all documents to avoid generation overhead
	b.Logf("Pre-generating %d documents...", totalDocs)
	docs := make([]map[string]interface{}, totalDocs)
	for i := 0; i < totalDocs; i++ {
		docs[i] = map[string]interface{}{
			"id":     fmt.Sprintf("doc_%d", i),
			"method": []string{"GET", "POST", "PUT", "DELETE"}[i%4],
			"status": 200 + (i%5)*100,
			"path":   []string{"/api/v1", "/api/v2", "/health", "/metrics"}[i%4],
		}
	}
	
	// Create worker pool
	var wg sync.WaitGroup
	jobChan := make(chan []map[string]interface{}, numWorkers*2)
	
	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for batch := range jobChan {
				// Create large batch for efficiency
				batchReq := index.NewBatch()
				
				for _, doc := range batch {
					batchReq.Index(doc["id"].(string), doc)
				}
				
				// Execute batch
				if err := index.Batch(batchReq); err != nil {
					b.Logf("Worker %d: Batch failed: %v", workerID, err)
				} else {
					count := atomic.AddInt64(&indexed, int64(len(batch)))
					if count%100000 == 0 {
						b.Logf("Progress: %d docs indexed", count)
					}
				}
			}
		}(w)
	}
	
	// Send batches to workers
	b.Logf("Starting indexing with %d workers, batch size %d", numWorkers, batchSize)
	for i := 0; i < len(docs); i += batchSize {
		end := i + batchSize
		if end > len(docs) {
			end = len(docs)
		}
		jobChan <- docs[i:end]
	}
	close(jobChan)
	
	wg.Wait()
	return int(indexed)
}

// benchmarkUltraOptimizedSearch tests search with optimizations
func benchmarkUltraOptimizedSearch(b *testing.B, index bleve.Index) {
	// Test cases based on Bleve docs recommendations
	testCases := []struct {
		name  string
		query query.Query
		size  int
	}{
		{
			name: "SimpleTermQuery_Small",
			query: func() query.Query {
				q := bleve.NewTermQuery("GET")
				q.SetField("method")
				return q
			}(),
			size: 10,  // Small size for pagination
		},
		{
			name: "NumericRange_DocValues",
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
			name: "MatchNone_Fast",  // Fastest possible query
			query: bleve.NewMatchNoneQuery(),
			size: 1,
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
				
				// Disable expensive features as per Bleve docs
				req.Explain = false
				req.IncludeLocations = false
				req.Highlight = nil
				req.Fields = []string{}  // Don't retrieve stored fields
				
				start := time.Now()
				result, err := index.Search(req)
				latency := time.Since(start).Microseconds()
				
				if err != nil {
					b.Errorf("Search failed: %v", err)
					continue
				}
				
				totalLatency += latency
				totalHits += result.Total
			}
			
			if b.N > 0 {
				avgLatency := totalLatency / int64(b.N)
				b.ReportMetric(float64(avgLatency), "μs/op")
				b.ReportMetric(float64(totalHits)/float64(b.N), "hits")
			}
		})
	}
}

// BenchmarkConcurrentUltraOptimized tests concurrent search performance
func BenchmarkConcurrentUltraOptimized(b *testing.B) {
	indexPath := filepath.Join(os.TempDir(), "bench_concurrent_ultra")
	defer os.RemoveAll(indexPath)
	
	// Create and populate small index for concurrent testing
	index, err := createUltraOptimizedIndex(indexPath)
	if err != nil {
		b.Fatalf("Failed to create index: %v", err)
	}
	defer index.Close()
	
	// Index smaller dataset quickly
	ultraFastBulkIndex(b, index, 100000, 10000, 8)
	time.Sleep(100 * time.Millisecond)
	
	// Test different concurrency levels
	for _, concurrency := range []int{1, 10, 50, 100, 500} {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			var totalQueries int64
			var totalLatency int64
			var wg sync.WaitGroup
			
			// Simple fast query
			q := bleve.NewTermQuery("GET")
			q.SetField("method")
			
			targetQueries := int64(1000)
			start := time.Now()
			
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					
					for atomic.LoadInt64(&totalQueries) < targetQueries {
						req := bleve.NewSearchRequest(q)
						req.Size = 1  // Minimal size
						req.From = 0
						req.Explain = false
						req.IncludeLocations = false
						
						queryStart := time.Now()
						_, err := index.Search(req)
						if err == nil {
							latency := time.Since(queryStart).Microseconds()
							atomic.AddInt64(&totalQueries, 1)
							atomic.AddInt64(&totalLatency, latency)
						}
					}
				}()
			}
			
			wg.Wait()
			duration := time.Since(start)
			
			queries := atomic.LoadInt64(&totalQueries)
			if queries > 0 {
				qps := float64(queries) / duration.Seconds()
				avgLatency := float64(totalLatency) / float64(queries)
				
				b.ReportMetric(qps, "qps")
				b.ReportMetric(avgLatency, "μs/query")
				b.Logf("Concurrency %d: %.0f qps, %.0fμs avg latency", concurrency, qps, avgLatency)
			}
		})
	}
}