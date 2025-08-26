package searcher

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
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
