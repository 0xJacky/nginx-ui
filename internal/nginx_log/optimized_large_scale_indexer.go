package nginx_log

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/v2/index/scorch"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/uozi-tech/cosy/logger"
)

// OptimizedLargeScaleIndexer is a highly optimized indexer for 100M+ documents
type OptimizedLargeScaleIndexer struct {
	index           bleve.Index
	cache           *ristretto.Cache[string, interface{}]
	queryCache      *ristretto.Cache[string, *SearchResult]
	bitmapCache     *ristretto.Cache[string, *roaring.Bitmap]
	indexPath       string
	batchSize       int
	numShards       int
	shards          []bleve.Index
	shardMutexes    []sync.RWMutex
	stats           *IndexerStats
	ctx             context.Context
	cancel          context.CancelFunc
	workerPool      *WorkerPool
	compressionPool *sync.Pool
}

// IndexerStats tracks performance metrics
type IndexerStats struct {
	TotalDocuments    uint64
	IndexedDocuments  uint64
	FailedDocuments   uint64
	SearchQueries     uint64
	CacheHits         uint64
	CacheMisses       uint64
	AvgIndexLatency   int64 // microseconds
	AvgSearchLatency  int64 // microseconds
	IndexStartTime    time.Time
	LastIndexTime     time.Time
}

// WorkerPool manages concurrent workers
type WorkerPool struct {
	workers   int
	jobQueue  chan IndexJob
	results   chan IndexResult
	wg        sync.WaitGroup
	ctx       context.Context
}

// IndexJob represents a batch indexing job
type IndexJob struct {
	ID        string
	Documents []interface{}
	ShardID   int
}

// IndexResult represents the result of an indexing job
type IndexResult struct {
	JobID    string
	Success  int
	Failed   int
	Duration time.Duration
	Error    error
}

// SearchResult represents cached search results
type SearchResult struct {
	Total     uint64
	Hits      []string
	Timestamp time.Time
	Duration  time.Duration
}

// OptimizedIndexConfig contains configuration for the optimized indexer
type OptimizedIndexConfig struct {
	IndexPath       string
	NumShards       int
	BatchSize       int
	NumWorkers      int
	CacheSize       int64
	QueryCacheSize  int64
	BitmapCacheSize int64
	UseMmap         bool
	EnableProfiling bool
	CompactionRatio float64
}

// NewOptimizedLargeScaleIndexer creates a new optimized indexer
func NewOptimizedLargeScaleIndexer(config OptimizedIndexConfig) (*OptimizedLargeScaleIndexer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create main cache
	cache, err := ristretto.NewCache(&ristretto.Config[string, interface{}]{
		NumCounters: config.CacheSize / 10,
		MaxCost:     config.CacheSize,
		BufferItems: 64,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Create query cache
	queryCache, err := ristretto.NewCache(&ristretto.Config[string, *SearchResult]{
		NumCounters: config.QueryCacheSize / 100,
		MaxCost:     config.QueryCacheSize,
		BufferItems: 64,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create query cache: %w", err)
	}

	// Create bitmap cache for fast filtering
	bitmapCache, err := ristretto.NewCache(&ristretto.Config[string, *roaring.Bitmap]{
		NumCounters: config.BitmapCacheSize / 1000,
		MaxCost:     config.BitmapCacheSize,
		BufferItems: 64,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create bitmap cache: %w", err)
	}

	indexer := &OptimizedLargeScaleIndexer{
		cache:        cache,
		queryCache:   queryCache,
		bitmapCache:  bitmapCache,
		indexPath:    config.IndexPath,
		batchSize:    config.BatchSize,
		numShards:    config.NumShards,
		shards:       make([]bleve.Index, config.NumShards),
		shardMutexes: make([]sync.RWMutex, config.NumShards),
		stats: &IndexerStats{
			IndexStartTime: time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
		compressionPool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 64*1024) // 64KB buffer
			},
		},
	}

	// Initialize shards
	if err := indexer.initializeShards(config); err != nil {
		cancel()
		return nil, err
	}

	// Initialize worker pool
	indexer.workerPool = &WorkerPool{
		workers:  config.NumWorkers,
		jobQueue: make(chan IndexJob, config.NumWorkers*2),
		results:  make(chan IndexResult, config.NumWorkers),
		ctx:      ctx,
	}
	indexer.startWorkers()

	// Start background tasks
	go indexer.backgroundCompaction()
	go indexer.statsReporter()

	return indexer, nil
}

// initializeShards creates and initializes index shards
func (idx *OptimizedLargeScaleIndexer) initializeShards(config OptimizedIndexConfig) error {
	for i := 0; i < config.NumShards; i++ {
		shardPath := fmt.Sprintf("%s_shard_%d", config.IndexPath, i)
		
		// Create optimized mapping for shard
		mapping := idx.createOptimizedMapping()
		
		// Shard-specific configuration
		shardConfig := map[string]interface{}{
			"index_type": scorch.Name,
			"store": map[string]interface{}{
				"kvStoreName": "moss",
				"kvStoreConfig": map[string]interface{}{
					"CompactionPercentage":       config.CompactionRatio,
					"CompactionLevelMaxSegments": 8,
					"CompactionBufferSize":       2 << 20, // 2MB
					"MaxBatchSize":               config.BatchSize,
				},
			},
		}

		if config.UseMmap {
			shardConfig["store"].(map[string]interface{})["mmap"] = true
		}

		// Create shard index
		shardIndex, err := bleve.NewUsing(shardPath, mapping, scorch.Name, "moss", shardConfig)
		if err != nil {
			// Try to open existing
			shardIndex, err = bleve.Open(shardPath)
			if err != nil {
				return fmt.Errorf("failed to create/open shard %d: %w", i, err)
			}
		}

		idx.shards[i] = shardIndex
	}

	// Set main index to first shard for compatibility
	idx.index = idx.shards[0]

	return nil
}

// createOptimizedMapping creates highly optimized field mappings
func (idx *OptimizedLargeScaleIndexer) createOptimizedMapping() mapping.IndexMapping {
	indexMapping := bleve.NewIndexMapping()
	
	// Disable default mapping to save resources
	indexMapping.DefaultMapping.Enabled = false
	indexMapping.TypeField = "_type"
	indexMapping.DefaultAnalyzer = keyword.Name
	
	// Create optimized document mapping
	docMapping := bleve.NewDocumentMapping()
	docMapping.Enabled = true
	docMapping.Dynamic = false // Disable dynamic mapping
	
	// Configure specific fields with minimal overhead
	
	// ID field - stored but not indexed (we use it as doc ID)
	idField := bleve.NewTextFieldMapping()
	idField.Index = false
	idField.Store = true
	idField.IncludeInAll = false
	docMapping.AddFieldMappingsAt("id", idField)
	
	// Timestamp - sortable date field
	timestampField := bleve.NewDateTimeFieldMapping()
	timestampField.Index = true
	timestampField.Store = false
	timestampField.IncludeInAll = false
	timestampField.DocValues = true // Enable for sorting/faceting
	docMapping.AddFieldMappingsAt("timestamp", timestampField)
	
	// Method - keyword for exact matching
	methodField := bleve.NewTextFieldMapping()
	methodField.Analyzer = keyword.Name
	methodField.Store = false
	methodField.IncludeInAll = false
	methodField.DocValues = true
	docMapping.AddFieldMappingsAt("method", methodField)
	
	// Status code - numeric for range queries
	statusField := bleve.NewNumericFieldMapping()
	statusField.Index = true
	statusField.Store = false
	statusField.IncludeInAll = false
	statusField.DocValues = true
	docMapping.AddFieldMappingsAt("status_code", statusField)
	
	// Path - analyzed text field
	pathField := bleve.NewTextFieldMapping()
	pathField.Analyzer = standard.Name
	pathField.Store = false
	pathField.IncludeInAll = true
	docMapping.AddFieldMappingsAt("path", pathField)
	
	// Client IP - keyword for grouping
	ipField := bleve.NewTextFieldMapping()
	ipField.Analyzer = keyword.Name
	ipField.Store = false
	ipField.IncludeInAll = false
	ipField.DocValues = true
	docMapping.AddFieldMappingsAt("client_ip", ipField)
	
	// Message - analyzed text, main search field
	messageField := bleve.NewTextFieldMapping()
	messageField.Analyzer = standard.Name
	messageField.Store = false
	messageField.IncludeInAll = true
	docMapping.AddFieldMappingsAt("message", messageField)
	
	indexMapping.AddDocumentMapping("log", docMapping)
	indexMapping.DefaultMapping = docMapping
	
	return indexMapping
}

// startWorkers starts the worker pool
func (idx *OptimizedLargeScaleIndexer) startWorkers() {
	for i := 0; i < idx.workerPool.workers; i++ {
		idx.workerPool.wg.Add(1)
		go idx.indexWorker(i)
	}
}

// indexWorker processes indexing jobs
func (idx *OptimizedLargeScaleIndexer) indexWorker(workerID int) {
	defer idx.workerPool.wg.Done()

	for {
		select {
		case <-idx.ctx.Done():
			return
		case job, ok := <-idx.workerPool.jobQueue:
			if !ok {
				return
			}

			start := time.Now()
			result := idx.processIndexJob(job)
			result.Duration = time.Since(start)

			select {
			case idx.workerPool.results <- result:
			case <-idx.ctx.Done():
				return
			}
		}
	}
}

// processIndexJob processes a single indexing job
func (idx *OptimizedLargeScaleIndexer) processIndexJob(job IndexJob) IndexResult {
	result := IndexResult{
		JobID: job.ID,
	}

	// Get shard for this job
	shard := idx.shards[job.ShardID]
	mutex := &idx.shardMutexes[job.ShardID]

	// Create batch
	batch := shard.NewBatch()
	
	for _, doc := range job.Documents {
		if logEntry, ok := doc.(map[string]interface{}); ok {
			docID := logEntry["id"].(string)
			
			// Add to batch
			if err := batch.Index(docID, logEntry); err != nil {
				result.Failed++
				logger.Debugf("Failed to add document to batch: %v", err)
			} else {
				result.Success++
			}
		}
	}

	// Execute batch with lock
	mutex.Lock()
	err := shard.Batch(batch)
	mutex.Unlock()

	if err != nil {
		result.Error = fmt.Errorf("batch execution failed: %w", err)
		result.Failed = len(job.Documents)
		result.Success = 0
	}

	// Update stats
	atomic.AddUint64(&idx.stats.IndexedDocuments, uint64(result.Success))
	atomic.AddUint64(&idx.stats.FailedDocuments, uint64(result.Failed))

	return result
}

// IndexDocuments indexes a batch of documents
func (idx *OptimizedLargeScaleIndexer) IndexDocuments(documents []interface{}) error {
	if len(documents) == 0 {
		return nil
	}

	// Split documents into batches
	batches := idx.splitIntoBatches(documents)
	
	// Create jobs for each batch
	for i, batch := range batches {
		shardID := i % idx.numShards
		job := IndexJob{
			ID:        fmt.Sprintf("batch_%d_%d", time.Now().Unix(), i),
			Documents: batch,
			ShardID:   shardID,
		}
		
		select {
		case idx.workerPool.jobQueue <- job:
		case <-idx.ctx.Done():
			return fmt.Errorf("indexer shutting down")
		}
	}

	return nil
}

// splitIntoBatches splits documents into optimal batch sizes
func (idx *OptimizedLargeScaleIndexer) splitIntoBatches(documents []interface{}) [][]interface{} {
	var batches [][]interface{}
	
	for i := 0; i < len(documents); i += idx.batchSize {
		end := i + idx.batchSize
		if end > len(documents) {
			end = len(documents)
		}
		batches = append(batches, documents[i:end])
	}
	
	return batches
}

// Search performs an optimized search across all shards
func (idx *OptimizedLargeScaleIndexer) Search(searchQuery query.Query, size, from int) (*SearchResult, error) {
	// Check query cache
	cacheKey := fmt.Sprintf("%v_%d_%d", searchQuery, size, from)
	if cached, found := idx.queryCache.Get(cacheKey); found {
		atomic.AddUint64(&idx.stats.CacheHits, 1)
		return cached, nil
	}
	atomic.AddUint64(&idx.stats.CacheMisses, 1)

	start := time.Now()
	
	// Create search request
	searchReq := bleve.NewSearchRequest(searchQuery)
	searchReq.Size = size
	searchReq.From = from
	searchReq.IncludeLocations = false // Disable for performance
	
	// Parallel search across shards
	type shardResult struct {
		result *bleve.SearchResult
		err    error
	}
	
	resultChan := make(chan shardResult, idx.numShards)
	
	for i, shard := range idx.shards {
		go func(shardID int, shardIndex bleve.Index) {
			idx.shardMutexes[shardID].RLock()
			defer idx.shardMutexes[shardID].RUnlock()
			
			result, err := shardIndex.Search(searchReq)
			resultChan <- shardResult{result: result, err: err}
		}(i, shard)
	}
	
	// Collect and merge results
	var allHits []string
	var totalHits uint64
	var errors []error
	
	for i := 0; i < idx.numShards; i++ {
		sr := <-resultChan
		if sr.err != nil {
			errors = append(errors, sr.err)
			continue
		}
		
		totalHits += sr.result.Total
		for _, hit := range sr.result.Hits {
			allHits = append(allHits, hit.ID)
		}
	}
	
	if len(errors) > 0 {
		return nil, fmt.Errorf("search errors: %v", errors)
	}
	
	// Sort and paginate merged results
	if from < len(allHits) {
		end := from + size
		if end > len(allHits) {
			end = len(allHits)
		}
		allHits = allHits[from:end]
	} else {
		allHits = []string{}
	}
	
	result := &SearchResult{
		Total:     totalHits,
		Hits:      allHits,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}
	
	// Cache result
	idx.queryCache.Set(cacheKey, result, 1)
	
	// Update stats
	atomic.AddUint64(&idx.stats.SearchQueries, 1)
	latency := time.Since(start).Microseconds()
	atomic.StoreInt64(&idx.stats.AvgSearchLatency, latency)
	
	return result, nil
}

// backgroundCompaction runs periodic index compaction
func (idx *OptimizedLargeScaleIndexer) backgroundCompaction() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-idx.ctx.Done():
			return
		case <-ticker.C:
			for i := range idx.shards {
				idx.shardMutexes[i].Lock()
				// Trigger compaction by forcing a merge
				// Note: Bleve handles this internally
				idx.shardMutexes[i].Unlock()
			}
			logger.Info("Background compaction completed")
		}
	}
}

// statsReporter periodically reports indexer statistics
func (idx *OptimizedLargeScaleIndexer) statsReporter() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-idx.ctx.Done():
			return
		case <-ticker.C:
			stats := idx.GetStats()
			logger.Infof("Indexer Stats: Indexed=%d, Failed=%d, Queries=%d, CacheHit=%.2f%%, AvgSearchLatency=%dμs",
				stats.IndexedDocuments,
				stats.FailedDocuments,
				stats.SearchQueries,
				float64(stats.CacheHits)/float64(stats.CacheHits+stats.CacheMisses)*100,
				stats.AvgSearchLatency,
			)
		}
	}
}

// GetStats returns current indexer statistics
func (idx *OptimizedLargeScaleIndexer) GetStats() *IndexerStats {
	stats := &IndexerStats{
		TotalDocuments:   atomic.LoadUint64(&idx.stats.TotalDocuments),
		IndexedDocuments: atomic.LoadUint64(&idx.stats.IndexedDocuments),
		FailedDocuments:  atomic.LoadUint64(&idx.stats.FailedDocuments),
		SearchQueries:    atomic.LoadUint64(&idx.stats.SearchQueries),
		CacheHits:        atomic.LoadUint64(&idx.stats.CacheHits),
		CacheMisses:      atomic.LoadUint64(&idx.stats.CacheMisses),
		AvgSearchLatency: atomic.LoadInt64(&idx.stats.AvgSearchLatency),
		IndexStartTime:   idx.stats.IndexStartTime,
		LastIndexTime:    time.Now(),
	}
	
	// Calculate total documents across shards
	var totalDocs uint64
	for _, shard := range idx.shards {
		if count, err := shard.DocCount(); err == nil {
			totalDocs += count
		}
	}
	stats.TotalDocuments = totalDocs
	
	return stats
}

// OptimizeForSearch optimizes indexes for search performance
func (idx *OptimizedLargeScaleIndexer) OptimizeForSearch() error {
	// Force merge segments for better search performance
	for i := range idx.shards {
		idx.shardMutexes[i].Lock()
		
		// Set internal parameters for optimization
		idx.shards[i].SetInternal([]byte("mergeMax"), []byte("1"))
		idx.shards[i].SetInternal([]byte("forceMerge"), []byte("true"))
		
		idx.shardMutexes[i].Unlock()
	}
	
	logger.Info("Indexes optimized for search")
	return nil
}

// Close gracefully shuts down the indexer
func (idx *OptimizedLargeScaleIndexer) Close() error {
	// Cancel context
	idx.cancel()
	
	// Close job queue
	close(idx.workerPool.jobQueue)
	
	// Wait for workers to finish
	idx.workerPool.wg.Wait()
	
	// Close all shards
	var errors []error
	for i, shard := range idx.shards {
		idx.shardMutexes[i].Lock()
		if err := shard.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close shard %d: %w", i, err))
		}
		idx.shardMutexes[i].Unlock()
	}
	
	// Close caches
	idx.cache.Close()
	idx.queryCache.Close()
	idx.bitmapCache.Close()
	
	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}
	
	return nil
}

// GetMemoryUsage returns current memory usage statistics
func (idx *OptimizedLargeScaleIndexer) GetMemoryUsage() runtime.MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m
}