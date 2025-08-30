package searcher

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/uozi-tech/cosy/logger"
)

// DistributedSearcher implements high-performance distributed search across multiple shards
type DistributedSearcher struct {
	config       *Config
	shards       []bleve.Index
	indexAlias   bleve.IndexAlias  // Index alias for global scoring
	queryBuilder *QueryBuilderService
	cache        *OptimizedSearchCache
	stats        *searcherStats

	// Concurrency control
	semaphore chan struct{}

	// State
	running int32
	
	// Cleanup control
	closeOnce sync.Once
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

	// Create index alias for global scoring across shards
	indexAlias := bleve.NewIndexAlias(shards...)
	
	// Set the index mapping from the first shard (all shards should have the same mapping)
	if len(shards) > 0 && shards[0] != nil {
		mapping := shards[0].Mapping()
		if err := indexAlias.SetIndexMapping(mapping); err != nil {
			// Log error but continue - this is not critical for basic functionality
		}
	}

	searcher := &DistributedSearcher{
		config:       config,
		shards:       shards,
		indexAlias:   indexAlias,
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

	// Use Bleve's native distributed search with global scoring for consistent pagination
	return ds.executeGlobalScoringSearch(ctx, query, req)
}

// executeGlobalScoringSearch uses Bleve's native distributed search with global scoring
// This ensures consistent pagination by letting Bleve handle cross-shard ranking
func (ds *DistributedSearcher) executeGlobalScoringSearch(ctx context.Context, query query.Query, req *SearchRequest) (*SearchResult, error) {
	// Create search request with proper pagination
	searchReq := bleve.NewSearchRequest(query)
	
	// Set pagination parameters directly - Bleve will handle distributed pagination correctly
	searchReq.Size = req.Limit
	if searchReq.Size <= 0 {
		searchReq.Size = 50 // Default page size
	}
	searchReq.From = req.Offset
	
	// Configure the search request with proper sorting and other settings
	ds.configureSearchRequest(searchReq, req)
	
	// Enable global scoring for distributed search consistency
	// This is the key fix from Bleve documentation for distributed search
	globalCtx := context.WithValue(ctx, search.SearchTypeKey, search.GlobalScoring)
	
	// Debug: Log the constructed query for comparison
	if queryBytes, err := json.Marshal(searchReq.Query); err == nil {
		logger.Debugf("Main search query: %s", string(queryBytes))
		logger.Debugf("Main search Size=%d, From=%d, Fields=%v", searchReq.Size, searchReq.From, searchReq.Fields)
	}
	
	// Execute search using Bleve's IndexAlias with global scoring
	result, err := ds.indexAlias.SearchInContext(globalCtx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("global scoring search failed: %w", err)
	}
	
	// Convert Bleve result to our SearchResult format
	return ds.convertBleveResult(result), nil
}

// convertBleveResult converts a Bleve SearchResult to our SearchResult format
func (ds *DistributedSearcher) convertBleveResult(bleveResult *bleve.SearchResult) *SearchResult {
	result := &SearchResult{
		Hits:      make([]*SearchHit, 0, len(bleveResult.Hits)),
		TotalHits: bleveResult.Total,
		MaxScore:  bleveResult.MaxScore,
		Facets:    make(map[string]*Facet),
	}
	
	// Convert hits
	for _, hit := range bleveResult.Hits {
		searchHit := &SearchHit{
			ID:           hit.ID,
			Score:        hit.Score,
			Fields:       hit.Fields,
			Highlighting: hit.Fragments,
			Index:        hit.Index,
		}
		result.Hits = append(result.Hits, searchHit)
	}
	
	// Convert facets if present
	for name, facet := range bleveResult.Facets {
		convertedFacet := &Facet{
			Field:   name,
			Total:   facet.Total,
			Missing: facet.Missing,
			Other:   facet.Other,
			Terms:   make([]*FacetTerm, 0),
		}
		
		if facet.Terms != nil {
			facetTerms := facet.Terms.Terms()
			convertedFacet.Terms = make([]*FacetTerm, 0, len(facetTerms))
			for _, term := range facetTerms {
				convertedFacet.Terms = append(convertedFacet.Terms, &FacetTerm{
					Term:  term.Term,
					Count: term.Count,
				})
			}
			
			// Fix Total to be the actual count of unique terms, not the sum
			// This addresses the issue where Bleve may incorrectly aggregate Total values
			// across multiple shards in IndexAlias
			convertedFacet.Total = len(facetTerms)
		} else {
			// If there are no terms, Total should be 0
			convertedFacet.Total = 0
		}
		
		result.Facets[name] = convertedFacet
	}
	
	return result
}

// configureSearchRequest sets up common search request configuration
func (ds *DistributedSearcher) configureSearchRequest(searchReq *bleve.SearchRequest, req *SearchRequest) {
	// Set up sorting with proper Bleve syntax
	sortField := req.SortBy
	if sortField == "" {
		sortField = "timestamp" // Default sort field
	}
	
	sortOrder := req.SortOrder
	if sortOrder == "" {
		sortOrder = SortOrderDesc // Default sort order
	}
	
	// Apply Bleve sorting - use "-" prefix for descending order
	if sortOrder == SortOrderDesc {
		searchReq.SortBy([]string{"-" + sortField})
	} else {
		searchReq.SortBy([]string{sortField})
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
}


// Utility methods

func (ds *DistributedSearcher) setRequestDefaults(req *SearchRequest) {
	if req.Timeout == 0 {
		req.Timeout = ds.config.TimeoutDuration
	}
	if req.UseCache && !ds.config.EnableCache {
		req.UseCache = false
	}
	if !req.UseCache && ds.config.EnableCache {
		req.UseCache = true
	}
}

func (ds *DistributedSearcher) getHealthyShards() []int {
	// With IndexAlias, Bleve handles shard health internally
	// Return all shard IDs since the alias will route correctly
	healthy := make([]int, len(ds.shards))
	for i := range ds.shards {
		healthy[i] = i
	}
	return healthy
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

// IsRunning returns true if the searcher is currently running
func (ds *DistributedSearcher) IsRunning() bool {
	return atomic.LoadInt32(&ds.running) == 1
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

// GetShards returns the underlying shards for cardinality counting
func (ds *DistributedSearcher) GetShards() []bleve.Index {
	return ds.shards
}

// SwapShards atomically replaces the current shards with new ones using IndexAlias.Swap()
// This follows Bleve best practices for zero-downtime index updates
func (ds *DistributedSearcher) SwapShards(newShards []bleve.Index) error {
	if atomic.LoadInt32(&ds.running) == 0 {
		return fmt.Errorf("searcher is not running")
	}

	if ds.indexAlias == nil {
		return fmt.Errorf("indexAlias is nil")
	}

	// Store old shards for logging
	oldShards := ds.shards
	
	// Perform atomic swap using IndexAlias - this is the key Bleve operation
	// that provides zero-downtime index updates
	logger.Debugf("SwapShards: Starting atomic swap - old=%d, new=%d", len(oldShards), len(newShards))
	
	swapStartTime := time.Now()
	ds.indexAlias.Swap(newShards, oldShards)
	swapDuration := time.Since(swapStartTime)
	
	logger.Infof("IndexAlias.Swap completed in %v (old=%d shards, new=%d shards)", 
		swapDuration, len(oldShards), len(newShards))
	
	// Update internal shards reference to match the IndexAlias
	ds.shards = newShards
	
	// Clear cache after shard swap to prevent stale results
	// Use goroutine to avoid potential deadlock during shard swap
	if ds.cache != nil {
		// Capture cache reference to avoid race condition
		cache := ds.cache
		go func() {
			// Add a small delay to ensure shard swap is fully completed
			time.Sleep(100 * time.Millisecond)
			
			// Double-check cache is still valid before clearing
			if cache != nil {
				cache.Clear()
				logger.Infof("Cache cleared after shard swap to prevent stale results")
			}
		}()
	}
	
	// Update shard stats for the new shards
	ds.stats.mutex.Lock()
	// Clear old shard stats
	ds.stats.shardStats = make(map[int]*ShardSearchStats)
	// Initialize stats for new shards
	for i := range newShards {
		ds.stats.shardStats[i] = &ShardSearchStats{
			ShardID:   i,
			IsHealthy: true,
		}
	}
	ds.stats.mutex.Unlock()
	
	logger.Infof("IndexAlias.Swap() completed: %d old shards -> %d new shards", 
		len(oldShards), len(newShards))
	
	// Verify each new shard's document count for debugging
	for i, shard := range newShards {
		if shard != nil {
			if docCount, err := shard.DocCount(); err != nil {
				logger.Warnf("New shard %d: error getting doc count: %v", i, err)
			} else {
				logger.Infof("New shard %d: contains %d documents", i, docCount)
			}
		} else {
			logger.Warnf("New shard %d: is nil", i)
		}
	}
	
	// Test the searcher with a simple query to verify functionality
	testCtx := context.Background()
	testReq := &SearchRequest{
		Limit:  1,
		Offset: 0,
	}
	
	if _, err := ds.Search(testCtx, testReq); err != nil {
		logger.Errorf("Post-swap searcher test query failed: %v", err)
		return fmt.Errorf("searcher test failed after shard swap: %w", err)
	} else {
		logger.Info("Post-swap searcher test query succeeded")
	}
	
	return nil
}

// Stop gracefully stops the searcher and closes all bleve indexes
func (ds *DistributedSearcher) Stop() error {
	var err error
	
	ds.closeOnce.Do(func() {
		// Set running to 0
		atomic.StoreInt32(&ds.running, 0)
		
		// Close the index alias first (this doesn't close underlying indexes)
		if ds.indexAlias != nil {
			if closeErr := ds.indexAlias.Close(); closeErr != nil {
				logger.Errorf("Failed to close index alias: %v", closeErr)
				err = closeErr
			}
			ds.indexAlias = nil
		}
		
		// DON'T close the underlying shards - they are managed by the indexer/shard manager
		// The searcher is just a consumer of these shards, not the owner
		// Clear the shards slice reference without closing the indexes
		ds.shards = nil
		
		// Close cache if it exists
		if ds.cache != nil {
			ds.cache.Close()
			ds.cache = nil
		}
	})
	
	return err
}
