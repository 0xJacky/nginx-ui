package searcher

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

// DistributedSearcher implements high-performance distributed search across multiple shards
type DistributedSearcher struct {
	config       *Config
	shards       []bleve.Index
	queryBuilder *QueryBuilderService
	cache        *OptimizedSearchCache
	stats        *searcherStats

	// Concurrency control
	semaphore chan struct{}

	// State
	running int32
}

// searcherStats tracks search performance metrics
type searcherStats struct {
	totalSearches      int64
	successfulSearches int64
	failedSearches     int64
	totalLatency       int64 // nanoseconds
	minLatency         int64
	maxLatency         int64
	activeSearches     int32

	shardStats map[int]*ShardSearchStats
	mutex      sync.RWMutex
}

// NewDistributedSearcher creates a new distributed searcher
func NewDistributedSearcher(config *Config, shards []bleve.Index) *DistributedSearcher {
	if config == nil {
		config = DefaultSearcherConfig()
	}

	searcher := &DistributedSearcher{
		config:       config,
		shards:       shards,
		queryBuilder: NewQueryBuilderService(),
		semaphore:    make(chan struct{}, config.MaxConcurrency),
		stats: &searcherStats{
			shardStats: make(map[int]*ShardSearchStats),
			minLatency: int64(time.Hour), // Start with high value
		},
		running: 1,
	}

	// Initialize cache if enabled
	if config.EnableCache {
		searcher.cache = NewOptimizedSearchCache(int64(config.CacheSize))
	}

	// Initialize shard stats
	for i := range shards {
		searcher.stats.shardStats[i] = &ShardSearchStats{
			ShardID:   i,
			IsHealthy: true,
		}
	}

	return searcher
}

// Search performs a distributed search across all shards
func (ds *DistributedSearcher) Search(ctx context.Context, req *SearchRequest) (*SearchResult, error) {
	if atomic.LoadInt32(&ds.running) == 0 {
		return nil, fmt.Errorf("searcher is not running")
	}

	startTime := time.Now()
	defer func() {
		ds.recordSearchMetrics(time.Since(startTime), true)
	}()

	// Validate request
	if err := ds.queryBuilder.ValidateSearchRequest(req); err != nil {
		return nil, fmt.Errorf("invalid search request: %w", err)
	}

	// Set defaults
	ds.setRequestDefaults(req)

	// Check cache if enabled
	if ds.config.EnableCache && req.UseCache {
		if cached := ds.getFromCache(req); cached != nil {
			cached.FromCache = true
			cached.CacheHit = true
			return cached, nil
		}
	}

	// Apply timeout
	searchCtx := ctx
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		searchCtx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	} else if ds.config.TimeoutDuration > 0 {
		var cancel context.CancelFunc
		searchCtx, cancel = context.WithTimeout(ctx, ds.config.TimeoutDuration)
		defer cancel()
	}

	// Acquire semaphore for concurrency control
	select {
	case ds.semaphore <- struct{}{}:
		defer func() { <-ds.semaphore }()
	case <-searchCtx.Done():
		return nil, fmt.Errorf("search timeout")
	}

	atomic.AddInt32(&ds.stats.activeSearches, 1)
	defer atomic.AddInt32(&ds.stats.activeSearches, -1)

	// Build query
	query, err := ds.queryBuilder.BuildQuery(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	// Execute search across shards
	result, err := ds.executeDistributedSearch(searchCtx, query, req)
	if err != nil {
		ds.recordSearchMetrics(time.Since(startTime), false)
		return nil, err
	}

	result.Duration = time.Since(startTime)

	// Cache result if enabled
	if ds.config.EnableCache && req.UseCache {
		ds.cacheResult(req, result)
	}

	return result, nil
}

// SearchAsync performs asynchronous search
func (ds *DistributedSearcher) SearchAsync(ctx context.Context, req *SearchRequest) (<-chan *SearchResult, <-chan error) {
	resultChan := make(chan *SearchResult, 1)
	errorChan := make(chan error, 1)

	go func() {
		defer close(resultChan)
		defer close(errorChan)

		result, err := ds.Search(ctx, req)
		if err != nil {
			errorChan <- err
		} else {
			resultChan <- result
		}
	}()

	return resultChan, errorChan
}

// executeDistributedSearch executes search across all healthy shards
func (ds *DistributedSearcher) executeDistributedSearch(ctx context.Context, query query.Query, req *SearchRequest) (*SearchResult, error) {
	healthyShards := ds.getHealthyShards()
	if len(healthyShards) == 0 {
		return nil, fmt.Errorf("no healthy shards available")
	}

	searchReq := bleve.NewSearchRequest(query)
	// Use a very large size or implement batching for dashboard requests
	if req.Limit == 0 {
		searchReq.Size = 10_000_000 // Very large limit for unlimited requests
	} else {
		searchReq.Size = req.Limit + req.Offset // Ensure we get enough data for pagination
	}
	searchReq.From = 0

	// Set up sorting with proper direction
	if req.SortBy != "" {
		sortField := req.SortBy
		if req.SortOrder == "desc" {
			sortField = "-" + sortField // Bleve uses "-" prefix for descending sort
		}
		searchReq.SortBy([]string{sortField})
	} else {
		// Default to timestamp descending if no sort specified
		searchReq.SortBy([]string{"-timestamp"})
	}

	// Configure highlighting
	if req.IncludeHighlighting && ds.config.EnableHighlighting {
		searchReq.Highlight = bleve.NewHighlight()
		if len(req.Fields) > 0 {
			for _, field := range req.Fields {
				searchReq.Highlight.AddField(field)
			}
		} else {
			searchReq.Highlight.AddField("*")
		}
	}

	// Configure facets
	if req.IncludeFacets && ds.config.EnableFaceting {
		facetFields := req.FacetFields
		if len(facetFields) == 0 {
			// Default facet fields
			facetFields = []string{"status", "method", "browser", "os", "device_type", "region_code"}
		}

		for _, field := range facetFields {
			size := DefaultFacetSize
			if req.FacetSize > 0 {
				size = req.FacetSize
			}
			facet := bleve.NewFacetRequest(field, size)
			searchReq.AddFacet(field, facet)
		}
	}

	// Configure fields to return
	if len(req.Fields) > 0 {
		searchReq.Fields = req.Fields
	} else {
		searchReq.Fields = []string{"*"}
	}

	// Execute searches in parallel
	shardResults := make(chan *bleve.SearchResult, len(healthyShards))
	errChan := make(chan error, len(healthyShards))
	var wg sync.WaitGroup

	for _, shardID := range healthyShards {
		wg.Add(1)
		go func(sid int) {
			defer wg.Done()
			shard := ds.shards[sid]
			if shard == nil {
				errChan <- fmt.Errorf("shard %d is nil", sid)
				return
			}
			result, err := shard.SearchInContext(ctx, searchReq)
			if err != nil {
				errChan <- fmt.Errorf("shard %d error: %w", sid, err)
				ds.markShardUnhealthy(sid, err)
				return
			}
			shardResults <- result
			ds.markShardHealthy(sid)
		}(shardID)
	}

	wg.Wait()
	close(errChan)
	close(shardResults)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}
	if len(errors) > 0 {
		// For simplicity, just return the first error. A more robust implementation might wrap all errors.
		return nil, errors[0]
	}

	// Convert channel to slice for merging
	resultsSlice := make([]*bleve.SearchResult, 0, len(shardResults))
	for result := range shardResults {
		resultsSlice = append(resultsSlice, result)
	}

	// Merge results from all shards
	mergedResult := ds.mergeShardResults(resultsSlice)

	// Perform a stable sort in-memory on the combined result set.
	// This is inefficient for large datasets but necessary for accurate cross-shard sorting.
	sort.SliceStable(mergedResult.Hits, func(i, j int) bool {
		// Handle sorting for different field types
		val1, ok1 := mergedResult.Hits[i].Fields[req.SortBy]
		val2, ok2 := mergedResult.Hits[j].Fields[req.SortBy]
		if !ok1 || !ok2 {
			return false // Cannot compare if fields are missing
		}

		// Assuming timestamp or other numeric fields for now
		fVal1, ok1 := val1.(float64)
		fVal2, ok2 := val2.(float64)
		if !ok1 || !ok2 {
			return false // Cannot compare non-numeric fields
		}

		if req.SortOrder == SortOrderDesc {
			return fVal1 > fVal2
		}
		return fVal1 < fVal2
	})

	// Manually apply pagination to the globally sorted list
	if req.Limit > 0 {
		start := req.Offset
		end := start + req.Limit

		if start >= len(mergedResult.Hits) {
			mergedResult.Hits = []*SearchHit{}
		} else {
			if end > len(mergedResult.Hits) {
				end = len(mergedResult.Hits)
			}
			mergedResult.Hits = mergedResult.Hits[start:end]
		}
	}

	return mergedResult, nil
}

// mergeShardResults merges results from multiple Bleve search results into a single SearchResult
func (ds *DistributedSearcher) mergeShardResults(shardResults []*bleve.SearchResult) *SearchResult {
	merged := &SearchResult{
		Hits:      make([]*SearchHit, 0),
		TotalHits: 0,
		MaxScore:  0,
		Facets:    make(map[string]*Facet),
	}

	for _, result := range shardResults {
		if result == nil {
			continue
		}
		merged.TotalHits += result.Total
		if result.MaxScore > merged.MaxScore {
			merged.MaxScore = result.MaxScore
		}

		// Merge hits
		for _, hit := range result.Hits {
			merged.Hits = append(merged.Hits, &SearchHit{
				ID:           hit.ID,
				Score:        hit.Score,
				Fields:       hit.Fields,
				Highlighting: hit.Fragments,
				Index:        hit.Index,
			})
		}

		// Merge facets
		for name, facet := range result.Facets {
			if _, ok := merged.Facets[name]; !ok {
				merged.Facets[name] = &Facet{
					Field: name,
					Total: 0,
					Terms: make([]*FacetTerm, 0),
				}
			}
			merged.Facets[name].Total += facet.Total
			merged.Facets[name].Missing += facet.Missing
			merged.Facets[name].Other += facet.Other

			// A map-based merge to correctly handle term counts across shards.
			termMap := make(map[string]*FacetTerm)
			// Prime the map with already merged terms
			for _, term := range merged.Facets[name].Terms {
				termMap[term.Term] = term
			}
			// Merge new terms from the current shard's facet result
			if facet.Terms != nil {
				for _, term := range facet.Terms.Terms() {
					if existing, ok := termMap[term.Term]; ok {
						existing.Count += term.Count
					} else {
						termMap[term.Term] = &FacetTerm{Term: term.Term, Count: term.Count}
					}
				}
			}

			// Convert map back to slice and sort
			newTerms := make([]*FacetTerm, 0, len(termMap))
			for _, term := range termMap {
				newTerms = append(newTerms, term)
			}
			sort.Slice(newTerms, func(i, j int) bool {
				return newTerms[i].Count > newTerms[j].Count
			})
			merged.Facets[name].Terms = newTerms
		}
	}

	return merged
}

// Utility methods

func (ds *DistributedSearcher) setRequestDefaults(req *SearchRequest) {
	if req.SortBy == "" {
		req.SortBy = "timestamp"
	}
	if req.SortOrder == "" {
		req.SortOrder = SortOrderDesc
	}
	if req.Timeout == 0 {
		req.Timeout = ds.config.TimeoutDuration
	}
	req.UseCache = ds.config.EnableCache
}

func (ds *DistributedSearcher) getHealthyShards() []int {
	var healthy []int
	ds.stats.mutex.RLock()
	for id, stat := range ds.stats.shardStats {
		if stat.IsHealthy {
			healthy = append(healthy, id)
		}
	}
	ds.stats.mutex.RUnlock()
	return healthy
}

func (ds *DistributedSearcher) markShardHealthy(shardID int) {
	ds.stats.mutex.Lock()
	if stat, exists := ds.stats.shardStats[shardID]; exists {
		stat.IsHealthy = true
		stat.LastSearchTime = time.Now()
	}
	ds.stats.mutex.Unlock()
}

func (ds *DistributedSearcher) markShardUnhealthy(shardID int, err error) {
	ds.stats.mutex.Lock()
	if stat, exists := ds.stats.shardStats[shardID]; exists {
		stat.IsHealthy = false
		stat.ErrorCount++
	}
	ds.stats.mutex.Unlock()
}

func (ds *DistributedSearcher) updateShardStats(shardID int, duration time.Duration, success bool) {
	ds.stats.mutex.Lock()
	if stat, exists := ds.stats.shardStats[shardID]; exists {
		stat.SearchCount++
		stat.LastSearchTime = time.Now()

		// Update average latency
		if stat.AverageLatency == 0 {
			stat.AverageLatency = duration
		} else {
			stat.AverageLatency = (stat.AverageLatency + duration) / 2
		}

		if !success {
			stat.ErrorCount++
		}
	}
	ds.stats.mutex.Unlock()
}

func (ds *DistributedSearcher) recordSearchMetrics(duration time.Duration, success bool) {
	atomic.AddInt64(&ds.stats.totalSearches, 1)
	atomic.AddInt64(&ds.stats.totalLatency, int64(duration))

	if success {
		atomic.AddInt64(&ds.stats.successfulSearches, 1)
	} else {
		atomic.AddInt64(&ds.stats.failedSearches, 1)
	}

	// Update min/max latency
	durationNs := int64(duration)
	for {
		current := atomic.LoadInt64(&ds.stats.minLatency)
		if durationNs >= current || atomic.CompareAndSwapInt64(&ds.stats.minLatency, current, durationNs) {
			break
		}
	}

	for {
		current := atomic.LoadInt64(&ds.stats.maxLatency)
		if durationNs <= current || atomic.CompareAndSwapInt64(&ds.stats.maxLatency, current, durationNs) {
			break
		}
	}
}

// Health and statistics

func (ds *DistributedSearcher) IsHealthy() bool {
	healthy := ds.getHealthyShards()
	return len(healthy) > 0
}

func (ds *DistributedSearcher) GetStats() *Stats {
	ds.stats.mutex.RLock()
	defer ds.stats.mutex.RUnlock()

	stats := &Stats{
		TotalSearches:      atomic.LoadInt64(&ds.stats.totalSearches),
		SuccessfulSearches: atomic.LoadInt64(&ds.stats.successfulSearches),
		FailedSearches:     atomic.LoadInt64(&ds.stats.failedSearches),
		ActiveSearches:     atomic.LoadInt32(&ds.stats.activeSearches),
		QueuedSearches:     len(ds.semaphore),
	}

	// Calculate average latency
	totalLatency := atomic.LoadInt64(&ds.stats.totalLatency)
	if stats.TotalSearches > 0 {
		stats.AverageLatency = time.Duration(totalLatency / stats.TotalSearches)
	}

	stats.MinLatency = time.Duration(atomic.LoadInt64(&ds.stats.minLatency))
	stats.MaxLatency = time.Duration(atomic.LoadInt64(&ds.stats.maxLatency))

	// Copy shard stats
	stats.ShardStats = make([]*ShardSearchStats, 0, len(ds.stats.shardStats))
	for _, stat := range ds.stats.shardStats {
		statCopy := *stat
		stats.ShardStats = append(stats.ShardStats, &statCopy)
	}

	// Add cache stats if cache is enabled
	if ds.cache != nil {
		stats.CacheStats = ds.cache.GetStats()
	}

	return stats
}

func (ds *DistributedSearcher) GetConfig() *Config {
	return ds.config
}

// Stop gracefully stops the searcher
func (ds *DistributedSearcher) Stop() error {
	atomic.StoreInt32(&ds.running, 0)
	return nil
}
