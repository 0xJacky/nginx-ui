package searcher

import (
	"context"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
)

// SearchResultPool provides efficient result reuse
type SearchResultPool struct {
	pool sync.Pool
}

// NewSearchResultPool creates a search result pool
func NewSearchResultPool() *SearchResultPool {
	return &SearchResultPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &SearchResult{
					Hits:   make([]*SearchHit, 0, 100),
					Facets: make(map[string]*Facet),
				}
			},
		},
	}
}

// Get retrieves a search result from pool
func (srp *SearchResultPool) Get() *SearchResult {
	result := srp.pool.Get().(*SearchResult)
	
	// Reset the result
	result.Hits = result.Hits[:0]
	result.TotalHits = 0
	result.MaxScore = 0
	result.Duration = 0
	result.FromCache = false
	
	// Clear maps
	for k := range result.Facets {
		delete(result.Facets, k)
	}
	
	return result
}

// Put returns a search result to pool
func (srp *SearchResultPool) Put(result *SearchResult) {
	// Don't keep overly large results in pool
	if cap(result.Hits) <= 1000 {
		srp.pool.Put(result)
	}
}

// OptimizedDistributedSearcher provides enhanced search performance
type OptimizedDistributedSearcher struct {
	*DistributedSearcher
	resultPool       *SearchResultPool
	queryCache       *QueryResultCache
	shardBalancer    *SearchLoadBalancer
	parallelizer     *SearchParallelizer
	resultAggregator *ResultAggregator
	memoryOptimizer  *SearchMemoryOptimizer
	perfMetrics      *SearchPerformanceMetrics
}

// NewOptimizedDistributedSearcher creates an optimized searcher
func NewOptimizedDistributedSearcher(config *SearcherConfig, shards []bleve.Index) *OptimizedDistributedSearcher {
	base := NewDistributedSearcher(config, shards)
	
	return &OptimizedDistributedSearcher{
		DistributedSearcher: base,
		resultPool:          NewSearchResultPool(),
		queryCache:          NewQueryResultCache(config.CacheSize * 2), // Larger cache
		shardBalancer:       NewSearchLoadBalancer(len(shards)),
		parallelizer:        NewSearchParallelizer(config.MaxConcurrency * 2),
		resultAggregator:    NewResultAggregator(),
		memoryOptimizer:     NewSearchMemoryOptimizer(),
		perfMetrics:         NewSearchPerformanceMetrics(),
	}
}

// Search performs optimized distributed search
func (ods *OptimizedDistributedSearcher) Search(ctx context.Context, req *SearchRequest) (*SearchResult, error) {
	startTime := time.Now()
	
	// Try cache first
	if ods.config.EnableCache {
		if cached := ods.queryCache.Get(req); cached != nil {
			ods.perfMetrics.RecordCacheHit()
			cached.FromCache = true
			return cached, nil
		}
		ods.perfMetrics.RecordCacheMiss()
	}
	
	// Perform optimized search
	result, err := ods.performOptimizedSearch(ctx, req)
	if err != nil {
		ods.perfMetrics.RecordError()
		return nil, err
	}
	
	// Cache result
	if ods.config.EnableCache && result != nil {
		ods.queryCache.Put(req, result, DefaultCacheTTL)
	}
	
	// Record metrics
	duration := time.Since(startTime)
	ods.perfMetrics.RecordSearch(len(result.Hits), duration)
	
	// Optimize memory usage
	ods.memoryOptimizer.CheckAndOptimize()
	
	return result, nil
}

// performOptimizedSearch executes the actual search with optimizations
func (ods *OptimizedDistributedSearcher) performOptimizedSearch(ctx context.Context, req *SearchRequest) (*SearchResult, error) {
	// Get optimal shard ordering
	shardOrder := ods.shardBalancer.GetOptimalShardOrder(req)
	
	// Execute parallel search
	shardResults, err := ods.parallelizer.ExecuteParallelSearch(ctx, req, ods.shards, shardOrder)
	if err != nil {
		return nil, err
	}
	
	// Aggregate results efficiently
	result := ods.resultAggregator.AggregateResults(shardResults, req)
	
	return result, nil
}

// QueryResultCache provides optimized query result caching
type QueryResultCache struct {
	cache        map[string]*CachedResult
	evictionList []*CacheEntryLRU
	maxSize      int
	currentSize  int
	mutex        sync.RWMutex
	hitCount     int64
	missCount    int64
}

// CachedResult represents a cached search result with metadata
type CachedResult struct {
	Result    *SearchResult
	ExpiresAt time.Time
	AccessCount int64
	LastAccess  time.Time
}

// CacheEntryLRU for LRU eviction (renamed to avoid conflict)
type CacheEntryLRU struct {
	Key        string
	AccessTime time.Time
}

// NewQueryResultCache creates an optimized cache
func NewQueryResultCache(maxSize int) *QueryResultCache {
	return &QueryResultCache{
		cache:        make(map[string]*CachedResult, maxSize),
		evictionList: make([]*CacheEntryLRU, 0, maxSize),
		maxSize:      maxSize,
	}
}

// Get retrieves a result from cache
func (qrc *QueryResultCache) Get(req *SearchRequest) *SearchResult {
	key := qrc.generateOptimizedKey(req)
	
	qrc.mutex.RLock()
	cached, exists := qrc.cache[key]
	qrc.mutex.RUnlock()
	
	if !exists {
		atomic.AddInt64(&qrc.missCount, 1)
		return nil
	}
	
	// Check expiration
	if time.Now().After(cached.ExpiresAt) {
		qrc.mutex.Lock()
		delete(qrc.cache, key)
		qrc.currentSize--
		qrc.mutex.Unlock()
		
		atomic.AddInt64(&qrc.missCount, 1)
		return nil
	}
	
	// Update access statistics
	atomic.AddInt64(&cached.AccessCount, 1)
	cached.LastAccess = time.Now()
	atomic.AddInt64(&qrc.hitCount, 1)
	
	return cached.Result
}

// Put stores a result in cache
func (qrc *QueryResultCache) Put(req *SearchRequest, result *SearchResult, ttl time.Duration) {
	key := qrc.generateOptimizedKey(req)
	
	qrc.mutex.Lock()
	defer qrc.mutex.Unlock()
	
	// Evict if necessary
	if qrc.currentSize >= qrc.maxSize {
		qrc.evictLRU()
	}
	
	cached := &CachedResult{
		Result:      result,
		ExpiresAt:   time.Now().Add(ttl),
		AccessCount: 1,
		LastAccess:  time.Now(),
	}
	
	qrc.cache[key] = cached
	qrc.currentSize++
	
	// Update eviction list
	qrc.evictionList = append(qrc.evictionList, &CacheEntryLRU{
		Key:        key,
		AccessTime: time.Now(),
	})
}

// evictLRU evicts least recently used entries
func (qrc *QueryResultCache) evictLRU() {
	if len(qrc.evictionList) == 0 {
		return
	}
	
	// Sort by access time and remove oldest 25%
	sort.Slice(qrc.evictionList, func(i, j int) bool {
		return qrc.evictionList[i].AccessTime.Before(qrc.evictionList[j].AccessTime)
	})
	
	evictCount := qrc.maxSize / 4
	if evictCount == 0 {
		evictCount = 1
	}
	
	for i := 0; i < evictCount && i < len(qrc.evictionList); i++ {
		key := qrc.evictionList[i].Key
		delete(qrc.cache, key)
		qrc.currentSize--
	}
	
	// Remove evicted entries from list
	qrc.evictionList = qrc.evictionList[evictCount:]
}

// generateOptimizedKey generates an efficient cache key
func (qrc *QueryResultCache) generateOptimizedKey(req *SearchRequest) string {
	// Use unsafe string building for performance
	var key []byte
	key = append(key, req.Query...)
	key = append(key, '|')
	
	// Convert numbers to bytes efficiently
	key = appendInt(key, req.Limit)
	key = append(key, '|')
	key = appendInt(key, req.Offset)
	key = append(key, '|')
	key = append(key, req.SortBy...)
	key = append(key, '|')
	key = append(key, req.SortOrder...)
	
	return BytesToStringUnsafe(key)
}

// SearchLoadBalancer optimizes shard selection for search queries
type SearchLoadBalancer struct {
	shardMetrics []ShardSearchMetrics
	totalShards  int
	mutex        sync.RWMutex
}

// ShardSearchMetrics tracks search performance per shard
type ShardSearchMetrics struct {
	AverageLatency time.Duration
	QueryCount     int64
	ErrorCount     int64
	LoadFactor     float64
	LastUpdate     time.Time
}

// NewSearchLoadBalancer creates a search load balancer
func NewSearchLoadBalancer(shardCount int) *SearchLoadBalancer {
	metrics := make([]ShardSearchMetrics, shardCount)
	for i := range metrics {
		metrics[i] = ShardSearchMetrics{
			LoadFactor: 1.0,
			LastUpdate: time.Now(),
		}
	}
	
	return &SearchLoadBalancer{
		shardMetrics: metrics,
		totalShards:  shardCount,
	}
}

// GetOptimalShardOrder returns optimal shard search order
func (slb *SearchLoadBalancer) GetOptimalShardOrder(req *SearchRequest) []int {
	slb.mutex.RLock()
	defer slb.mutex.RUnlock()
	
	// Create shard order based on load factors
	shardOrder := make([]int, slb.totalShards)
	for i := 0; i < slb.totalShards; i++ {
		shardOrder[i] = i
	}
	
	// Sort by load factor (ascending - less loaded shards first)
	sort.Slice(shardOrder, func(i, j int) bool {
		return slb.shardMetrics[shardOrder[i]].LoadFactor < slb.shardMetrics[shardOrder[j]].LoadFactor
	})
	
	return shardOrder
}

// RecordShardSearch records search metrics for a shard
func (slb *SearchLoadBalancer) RecordShardSearch(shardID int, duration time.Duration, success bool) {
	if shardID < 0 || shardID >= len(slb.shardMetrics) {
		return
	}
	
	slb.mutex.Lock()
	defer slb.mutex.Unlock()
	
	metric := &slb.shardMetrics[shardID]
	
	// Update average latency using exponential moving average
	if metric.AverageLatency == 0 {
		metric.AverageLatency = duration
	} else {
		alpha := 0.2 // Smoothing factor
		metric.AverageLatency = time.Duration(float64(metric.AverageLatency)*(1-alpha) + float64(duration)*alpha)
	}
	
	atomic.AddInt64(&metric.QueryCount, 1)
	if !success {
		atomic.AddInt64(&metric.ErrorCount, 1)
	}
	
	// Calculate load factor based on latency and error rate
	baseLoad := float64(metric.AverageLatency) / float64(time.Millisecond)
	errorRate := float64(metric.ErrorCount) / float64(metric.QueryCount)
	metric.LoadFactor = baseLoad * (1 + errorRate*10) // Penalize errors heavily
	
	metric.LastUpdate = time.Now()
}

// SearchParallelizer manages parallel search execution
type SearchParallelizer struct {
	semaphore   chan struct{}
	workerPool  *SearchWorkerPool
	maxRoutines int
}

// NewSearchParallelizer creates a search parallelizer
func NewSearchParallelizer(maxConcurrency int) *SearchParallelizer {
	return &SearchParallelizer{
		semaphore:   make(chan struct{}, maxConcurrency),
		workerPool:  NewSearchWorkerPool(maxConcurrency),
		maxRoutines: maxConcurrency,
	}
}

// ExecuteParallelSearch executes search across shards in parallel
func (sp *SearchParallelizer) ExecuteParallelSearch(ctx context.Context, req *SearchRequest, shards []bleve.Index, shardOrder []int) ([]*ShardSearchResult, error) {
	results := make([]*ShardSearchResult, len(shards))
	errors := make([]error, len(shards))
	
	var wg sync.WaitGroup
	
	for i, shardIdx := range shardOrder {
		wg.Add(1)
		
		// Acquire semaphore
		sp.semaphore <- struct{}{}
		
		go func(idx, shardIndex int) {
			defer wg.Done()
			defer func() { <-sp.semaphore }() // Release semaphore
			
			// Execute search on shard
			startTime := time.Now()
			result, err := sp.executeShardSearch(ctx, req, shards[shardIndex])
			duration := time.Since(startTime)
			
			results[idx] = &ShardSearchResult{
				ShardID:  shardIndex,
				Result:   result,
				Duration: duration,
				Error:    err,
			}
			errors[idx] = err
		}(i, shardIdx)
	}
	
	wg.Wait()
	
	// Check for critical errors
	errorCount := 0
	for _, err := range errors {
		if err != nil {
			errorCount++
		}
	}
	
	// If more than half the shards failed, return error
	if errorCount > len(shards)/2 {
		return nil, errors[0] // Return first error
	}
	
	return results, nil
}

// executeShardSearch executes search on a single shard
func (sp *SearchParallelizer) executeShardSearch(ctx context.Context, req *SearchRequest, shard bleve.Index) (*bleve.SearchResult, error) {
	// Convert SearchRequest to bleve.SearchRequest
	bleveReq := sp.convertToBlueveRequest(req)
	
	// Execute search with timeout
	return shard.SearchInContext(ctx, bleveReq)
}

// convertToBlueveRequest converts our SearchRequest to bleve.SearchRequest
func (sp *SearchParallelizer) convertToBlueveRequest(req *SearchRequest) *bleve.SearchRequest {
	bleveReq := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
	bleveReq.Size = req.Limit
	bleveReq.From = req.Offset
	
	// Add more sophisticated query conversion here
	return bleveReq
}

// SearchWorkerPool manages search workers
type SearchWorkerPool struct {
	workers   []*SearchWorker
	workQueue chan *SearchTask
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// SearchWorker represents a search worker
type SearchWorker struct {
	ID           int
	processedTasks int64
	errorCount    int64
}

// SearchTask represents a search task
type SearchTask struct {
	ShardID    int
	Request    *SearchRequest
	Shard      bleve.Index
	ResultChan chan *ShardSearchResult
}

// ShardSearchResult represents result from a single shard
type ShardSearchResult struct {
	ShardID  int
	Result   *bleve.SearchResult
	Duration time.Duration
	Error    error
}

// NewSearchWorkerPool creates a search worker pool
func NewSearchWorkerPool(numWorkers int) *SearchWorkerPool {
	pool := &SearchWorkerPool{
		workers:   make([]*SearchWorker, numWorkers),
		workQueue: make(chan *SearchTask, numWorkers*2),
		stopChan:  make(chan struct{}),
	}
	
	for i := 0; i < numWorkers; i++ {
		worker := &SearchWorker{ID: i}
		pool.workers[i] = worker
		
		pool.wg.Add(1)
		go pool.runSearchWorker(worker)
	}
	
	return pool
}

// runSearchWorker runs a single search worker
func (swp *SearchWorkerPool) runSearchWorker(worker *SearchWorker) {
	defer swp.wg.Done()
	
	for {
		select {
		case task := <-swp.workQueue:
			startTime := time.Now()
			result, err := task.Shard.SearchInContext(context.Background(), swp.convertRequest(task.Request))
			duration := time.Since(startTime)
			
			task.ResultChan <- &ShardSearchResult{
				ShardID:  task.ShardID,
				Result:   result,
				Duration: duration,
				Error:    err,
			}
			
			if err != nil {
				atomic.AddInt64(&worker.errorCount, 1)
			} else {
				atomic.AddInt64(&worker.processedTasks, 1)
			}
			
		case <-swp.stopChan:
			return
		}
	}
}

// convertRequest converts SearchRequest to bleve.SearchRequest
func (swp *SearchWorkerPool) convertRequest(req *SearchRequest) *bleve.SearchRequest {
	// Simplified conversion - in practice, this would be more sophisticated
	var q query.Query = bleve.NewMatchAllQuery()
	if req.Query != "" {
		q = bleve.NewMatchQuery(req.Query)
	}
	
	bleveReq := bleve.NewSearchRequest(q)
	bleveReq.Size = req.Limit
	bleveReq.From = req.Offset
	
	return bleveReq
}

// Close closes the worker pool
func (swp *SearchWorkerPool) Close() {
	close(swp.stopChan)
	swp.wg.Wait()
}

// ResultAggregator efficiently aggregates search results from multiple shards
type ResultAggregator struct {
	hitPool    sync.Pool
	resultPool *SearchResultPool
}

// NewResultAggregator creates a result aggregator
func NewResultAggregator() *ResultAggregator {
	return &ResultAggregator{
		hitPool: sync.Pool{
			New: func() interface{} {
				return &SearchHit{
					Fields: make(map[string]interface{}),
				}
			},
		},
		resultPool: NewSearchResultPool(),
	}
}

// AggregateResults aggregates results from multiple shards
func (ra *ResultAggregator) AggregateResults(shardResults []*ShardSearchResult, req *SearchRequest) *SearchResult {
	result := ra.resultPool.Get()
	
	var allHits []*SearchHit
	var totalHits int64
	var maxScore float64
	
	// Collect all hits from shards
	for _, shardResult := range shardResults {
		if shardResult.Error != nil || shardResult.Result == nil {
			continue
		}
		
		bleveResult := shardResult.Result
		totalHits += int64(bleveResult.Total)
		
		if bleveResult.MaxScore > maxScore {
			maxScore = bleveResult.MaxScore
		}
		
		// Convert bleve hits to our format
		for _, bleveHit := range bleveResult.Hits {
			hit := ra.convertBleveHit(bleveHit)
			allHits = append(allHits, hit)
		}
	}
	
	// Sort and paginate results
	sortedHits := ra.sortAndPaginateHits(allHits, req)
	
	result.Hits = sortedHits
	result.TotalHits = uint64(totalHits)
	result.MaxScore = maxScore
	
	return result
}

// convertBleveHit converts bleve DocumentMatch to SearchHit
func (ra *ResultAggregator) convertBleveHit(bleveHit *search.DocumentMatch) *SearchHit {
	hit := ra.hitPool.Get().(*SearchHit)
	
	hit.ID = bleveHit.ID
	hit.Score = bleveHit.Score
	
	// Clear and populate fields
	for k := range hit.Fields {
		delete(hit.Fields, k)
	}
	for k, v := range bleveHit.Fields {
		hit.Fields[k] = v
	}
	
	return hit
}

// sortAndPaginateHits sorts hits and applies pagination
func (ra *ResultAggregator) sortAndPaginateHits(hits []*SearchHit, req *SearchRequest) []*SearchHit {
	// Sort by score (descending by default)
	sort.Slice(hits, func(i, j int) bool {
		if req.SortOrder == SortOrderAsc {
			return hits[i].Score < hits[j].Score
		}
		return hits[i].Score > hits[j].Score
	})
	
	// Apply pagination
	start := req.Offset
	end := req.Offset + req.Limit
	
	if start > len(hits) {
		return []*SearchHit{}
	}
	if end > len(hits) {
		end = len(hits)
	}
	
	return hits[start:end]
}

// SearchMemoryOptimizer optimizes memory usage during searches
type SearchMemoryOptimizer struct {
	lastGC        time.Time
	gcThreshold   int64
	memStats      runtime.MemStats
	forceGCEnabled bool
}

// NewSearchMemoryOptimizer creates a memory optimizer
func NewSearchMemoryOptimizer() *SearchMemoryOptimizer {
	return &SearchMemoryOptimizer{
		gcThreshold:    512 * 1024 * 1024, // 512MB
		forceGCEnabled: true,
	}
}

// CheckAndOptimize checks memory usage and optimizes if necessary
func (smo *SearchMemoryOptimizer) CheckAndOptimize() {
	if !smo.forceGCEnabled {
		return
	}
	
	runtime.ReadMemStats(&smo.memStats)
	
	// Force GC if memory usage is high and enough time has passed
	if smo.memStats.Alloc > uint64(smo.gcThreshold) && time.Since(smo.lastGC) > 60*time.Second {
		runtime.GC()
		smo.lastGC = time.Now()
	}
}

// SearchPerformanceMetrics tracks search performance
type SearchPerformanceMetrics struct {
	totalSearches    int64
	totalHits        int64
	totalDuration    int64 // nanoseconds
	cacheHits        int64
	cacheMisses      int64
	errorCount       int64
	averageLatency   int64 // nanoseconds
	mutex            sync.RWMutex
}

// NewSearchPerformanceMetrics creates performance metrics tracker
func NewSearchPerformanceMetrics() *SearchPerformanceMetrics {
	return &SearchPerformanceMetrics{}
}

// RecordSearch records search metrics
func (spm *SearchPerformanceMetrics) RecordSearch(hitCount int, duration time.Duration) {
	atomic.AddInt64(&spm.totalSearches, 1)
	atomic.AddInt64(&spm.totalHits, int64(hitCount))
	atomic.AddInt64(&spm.totalDuration, int64(duration))
	
	// Update average latency
	searches := atomic.LoadInt64(&spm.totalSearches)
	totalDur := atomic.LoadInt64(&spm.totalDuration)
	atomic.StoreInt64(&spm.averageLatency, totalDur/searches)
}

// RecordCacheHit records cache hit
func (spm *SearchPerformanceMetrics) RecordCacheHit() {
	atomic.AddInt64(&spm.cacheHits, 1)
}

// RecordCacheMiss records cache miss
func (spm *SearchPerformanceMetrics) RecordCacheMiss() {
	atomic.AddInt64(&spm.cacheMisses, 1)
}

// RecordError records search error
func (spm *SearchPerformanceMetrics) RecordError() {
	atomic.AddInt64(&spm.errorCount, 1)
}

// GetMetrics returns performance metrics snapshot
func (spm *SearchPerformanceMetrics) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"total_searches":       atomic.LoadInt64(&spm.totalSearches),
		"total_hits":          atomic.LoadInt64(&spm.totalHits),
		"average_latency_ms":  float64(atomic.LoadInt64(&spm.averageLatency)) / 1e6,
		"cache_hits":          atomic.LoadInt64(&spm.cacheHits),
		"cache_misses":        atomic.LoadInt64(&spm.cacheMisses),
		"cache_hit_rate":      spm.getCacheHitRate(),
		"error_count":         atomic.LoadInt64(&spm.errorCount),
		"searches_per_second": spm.getSearchRate(),
	}
}

// getCacheHitRate calculates cache hit rate
func (spm *SearchPerformanceMetrics) getCacheHitRate() float64 {
	hits := atomic.LoadInt64(&spm.cacheHits)
	misses := atomic.LoadInt64(&spm.cacheMisses)
	total := hits + misses
	
	if total == 0 {
		return 0
	}
	
	return float64(hits) / float64(total)
}

// getSearchRate calculates searches per second
func (spm *SearchPerformanceMetrics) getSearchRate() float64 {
	searches := atomic.LoadInt64(&spm.totalSearches)
	duration := atomic.LoadInt64(&spm.totalDuration)
	
	if duration == 0 {
		return 0
	}
	
	return float64(searches) / (float64(duration) / 1e9)
}

// Utility functions
func appendInt(b []byte, i int) []byte {
	// Convert int to bytes efficiently
	if i == 0 {
		return append(b, '0')
	}
	
	// Handle negative numbers
	if i < 0 {
		b = append(b, '-')
		i = -i
	}
	
	// Convert digits
	start := len(b)
	for i > 0 {
		b = append(b, byte('0'+(i%10)))
		i /= 10
	}
	
	// Reverse the digits
	for i, j := start, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	
	return b
}

// BytesToStringUnsafe converts bytes to string without allocation
func BytesToStringUnsafe(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// StringToBytesUnsafe converts string to bytes without allocation
func StringToBytesUnsafe(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(
		&struct {
			string
			int
		}{s, len(s)},
	))
}