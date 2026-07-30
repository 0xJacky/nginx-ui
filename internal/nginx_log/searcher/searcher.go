package searcher

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/uozi-tech/cosy/logger"
)

// Searcher implements high-performance distributed search across multiple shards
type Searcher struct {
	config       *Config
	shards       []bleve.Index
	indexAlias   bleve.IndexAlias // Index alias for global scoring
	queryBuilder *QueryBuilder
	cache        *Cache
	stats        *searcherStats
	memoryLimit  *searchMemoryLimiter

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

// NewSearcher creates a new distributed searcher
func NewSearcher(config *Config, shards []bleve.Index) *Searcher {
	if config == nil {
		config = DefaultSearcherConfig()
	}
	shards = wrapRecoveringIndexes(shards)

	// Create index alias for global scoring across shards
	indexAlias := bleve.NewIndexAlias(shards...)

	// Set the index mapping from the first shard (all shards should have the same mapping)
	if len(shards) > 0 && shards[0] != nil {
		mapping := shards[0].Mapping()
		if err := indexAlias.SetIndexMapping(mapping); err != nil {
			// Log error but continue - this is not critical for basic functionality
		}
	}

	searcher := &Searcher{
		config:       config,
		shards:       shards,
		indexAlias:   indexAlias,
		queryBuilder: NewQueryBuilder(),
		memoryLimit:  newSearchMemoryLimiter(config.MemoryQuota),
		semaphore:    make(chan struct{}, config.MaxConcurrency),
		stats: &searcherStats{
			shardStats: make(map[int]*ShardSearchStats),
			minLatency: int64(time.Hour), // Start with high value
		},
		running: 1,
	}

	// Initialize cache if enabled
	if config.EnableCache {
		searcher.cache = NewCache(int64(config.CacheSize))
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
func (s *Searcher) Search(ctx context.Context, req *SearchRequest) (*SearchResult, error) {
	if atomic.LoadInt32(&s.running) == 0 {
		return nil, fmt.Errorf("searcher is not running")
	}

	startTime := time.Now()
	defer func() {
		s.recordSearchMetrics(time.Since(startTime), true)
	}()

	// Validate request
	if err := s.queryBuilder.ValidateSearchRequest(req); err != nil {
		return nil, fmt.Errorf("invalid search request: %w", err)
	}

	// Set defaults
	s.setRequestDefaults(req)

	// Check cache if enabled
	if s.config.EnableCache && req.UseCache {
		if cached := s.getFromCache(req); cached != nil {
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
	} else if s.config.TimeoutDuration > 0 {
		var cancel context.CancelFunc
		searchCtx, cancel = context.WithTimeout(ctx, s.config.TimeoutDuration)
		defer cancel()
	}
	searchCtx = withSearchMemoryLimit(searchCtx, s.memoryLimit)

	// Acquire semaphore for concurrency control
	select {
	case s.semaphore <- struct{}{}:
		defer func() { <-s.semaphore }()
	case <-searchCtx.Done():
		return nil, fmt.Errorf("search timeout")
	}

	atomic.AddInt32(&s.stats.activeSearches, 1)
	defer atomic.AddInt32(&s.stats.activeSearches, -1)

	// Build query
	query, err := s.queryBuilder.BuildQuery(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}
	indexAlias, releaseAlias := s.indexAliasForRequest(req)
	defer releaseAlias()

	// Execute search across shards
	result, err := s.executeDistributedSearch(searchCtx, query, req, indexAlias)
	if err != nil {
		s.recordSearchMetrics(time.Since(startTime), false)
		return nil, err
	}

	// Byte and response-time totals need their own pass over the match set:
	// the hits carry only the requested page, so summing them would describe
	// the page rather than the query.
	if req.IncludeStats {
		result.Stats = s.searchStats(searchCtx, query, req, result.TotalHits, indexAlias)
	}

	result.Duration = time.Since(startTime)

	// Cache result if enabled
	if s.config.EnableCache && req.UseCache {
		s.cacheResult(req, result)
	}

	return result, nil
}

// executeDistributedSearch executes search across all healthy shards
func (s *Searcher) executeDistributedSearch(
	ctx context.Context,
	query query.Query,
	req *SearchRequest,
	indexAlias bleve.IndexAlias,
) (*SearchResult, error) {
	healthyShards := s.getHealthyShards()
	if len(healthyShards) == 0 {
		return nil, fmt.Errorf("no healthy shards available")
	}

	// Use Bleve's native distributed search with global scoring for consistent pagination
	return s.executeGlobalScoringSearch(ctx, query, req, indexAlias)
}

// executeGlobalScoringSearch uses Bleve's native distributed search with global scoring
// This ensures consistent pagination by letting Bleve handle cross-shard ranking
func (s *Searcher) executeGlobalScoringSearch(
	ctx context.Context,
	query query.Query,
	req *SearchRequest,
	indexAlias bleve.IndexAlias,
) (*SearchResult, error) {
	// Create search request with proper pagination
	searchReq := bleve.NewSearchRequest(query)

	// Set pagination parameters directly - Bleve will handle distributed pagination correctly.
	// A negative Limit explicitly requests zero hits (facet/aggregation-only queries);
	// zero means "unset" and falls back to the default page size.
	switch {
	case req.Limit < 0:
		searchReq.Size = 0
	case req.Limit == 0:
		searchReq.Size = 50 // Default page size
	default:
		searchReq.Size = req.Limit
	}
	searchReq.From = req.Offset

	// Configure the search request with proper sorting and other settings
	s.configureSearchRequest(searchReq, req)

	// Global scoring runs an extra pre-search round across all shards to gather
	// term statistics for consistent TF-IDF ranking. That only matters when
	// results are ranked by relevance score; all other sort orders (the default
	// timestamp sort, facet-only queries) don't read scores, so skip the overhead.
	searchCtx := ctx
	if req.SortBy == "_score" {
		searchCtx = context.WithValue(ctx, search.SearchTypeKey, search.GlobalScoring)
	}

	// Execute search using Bleve's IndexAlias
	result, err := indexAlias.SearchInContext(searchCtx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("distributed search failed: %w", err)
	}
	if err := searchResultError(result); err != nil {
		return nil, err
	}

	// Convert Bleve result to our SearchResult format
	return s.convertBleveResult(result), nil
}

func (s *Searcher) indexAliasForRequest(req *SearchRequest) (bleve.IndexAlias, func()) {
	return indexAliasForLogPaths(s.indexAlias, s.shards, req.UseMainLogPath, req.LogPaths)
}

// convertBleveResult converts a Bleve SearchResult to our SearchResult format
func (s *Searcher) convertBleveResult(bleveResult *bleve.SearchResult) *SearchResult {
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
			Sort:         hit.Sort,
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
		} else if len(facet.NumericRanges) > 0 {
			// Numeric range facets (e.g. the numeric status field) report their
			// buckets as NumericRanges. Expose non-empty ranges as terms so all
			// facets can be consumed uniformly.
			convertedFacet.Terms = make([]*FacetTerm, 0, len(facet.NumericRanges))
			for _, nr := range facet.NumericRanges {
				if nr.Count == 0 {
					continue
				}
				convertedFacet.Terms = append(convertedFacet.Terms, &FacetTerm{
					Term:  nr.Name,
					Count: nr.Count,
				})
			}
			convertedFacet.Total = len(convertedFacet.Terms)
		} else {
			// If there are no terms, Total should be 0
			convertedFacet.Total = 0
		}

		result.Facets[name] = convertedFacet
	}

	return result
}

// configureSearchRequest sets up common search request configuration
func (s *Searcher) configureSearchRequest(searchReq *bleve.SearchRequest, req *SearchRequest) {
	// Set up sorting with proper Bleve syntax
	sortField := req.SortBy
	if sortField == "" {
		sortField = "timestamp" // Default sort field
	}

	sortOrder := req.SortOrder
	if sortOrder == "" {
		sortOrder = SortOrderDesc // Default sort order
	}

	// Apply Bleve sorting - use "-" prefix for descending order.
	// Always append the document ID as a final tiebreaker: the primary sort
	// key (usually second-resolution timestamps) is not unique, and a
	// deterministic total order is required for stable pagination and
	// SearchAfter cursors.
	if sortOrder == SortOrderDesc {
		searchReq.SortBy([]string{"-" + sortField, "_id"})
	} else {
		searchReq.SortBy([]string{sortField, "_id"})
	}

	// Cursor-based pagination: resume strictly after the given sort values
	if len(req.SearchAfter) > 0 {
		searchReq.SearchAfter = req.SearchAfter
		searchReq.From = 0 // SearchAfter and From are mutually exclusive
	}

	// Configure highlighting
	if req.IncludeHighlighting && s.config.EnableHighlighting {
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
	if req.IncludeFacets && s.config.EnableFaceting {
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
			// The status field is indexed as numeric, so a terms facet cannot
			// bucket it. Add one numeric range per HTTP status code instead.
			if field == "status" {
				addStatusCodeRanges(facet)
			}
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

// statusCodeFacetRange is a precomputed numeric facet bucket for one HTTP
// status code.
type statusCodeFacetRange struct {
	name string
	min  float64
	max  float64
}

// statusCodeFacetRanges holds one numeric range per HTTP status code (100-599).
// It is built once at startup so that faceting the numeric status field does
// not rebuild the full range set on every search request.
var statusCodeFacetRanges = func() []statusCodeFacetRange {
	ranges := make([]statusCodeFacetRange, 0, 500)
	for code := 100; code <= 599; code++ {
		ranges = append(ranges, statusCodeFacetRange{
			name: strconv.Itoa(code),
			min:  float64(code),
			max:  float64(code + 1),
		})
	}
	return ranges
}()

// addStatusCodeRanges attaches one numeric range per HTTP status code to the
// facet request. The status field is indexed as numeric, so it must be faceted
// with numeric ranges rather than terms; each range is named after the status
// code so the converted facet exposes the code itself as the term.
func addStatusCodeRanges(facet *bleve.FacetRequest) {
	for i := range statusCodeFacetRanges {
		r := &statusCodeFacetRanges[i]
		facet.AddNumericRange(r.name, &r.min, &r.max)
	}
}

// Utility methods

func (s *Searcher) setRequestDefaults(req *SearchRequest) {
	if req.Timeout == 0 {
		req.Timeout = s.config.TimeoutDuration
	}
	// Only downgrade: callers may opt out of caching (e.g. one-off batch scans
	// whose results would churn the cache), so never force UseCache back on.
	if req.UseCache && !s.config.EnableCache {
		req.UseCache = false
	}
}

func (s *Searcher) getHealthyShards() []int {
	// With IndexAlias, Bleve handles shard health internally
	// Return all shard IDs since the alias will route correctly
	healthy := make([]int, len(s.shards))
	for i := range s.shards {
		healthy[i] = i
	}
	return healthy
}

func (s *Searcher) recordSearchMetrics(duration time.Duration, success bool) {
	atomic.AddInt64(&s.stats.totalSearches, 1)
	atomic.AddInt64(&s.stats.totalLatency, int64(duration))

	if success {
		atomic.AddInt64(&s.stats.successfulSearches, 1)
	} else {
		atomic.AddInt64(&s.stats.failedSearches, 1)
	}

	// Update min/max latency
	durationNs := int64(duration)
	for {
		current := atomic.LoadInt64(&s.stats.minLatency)
		if durationNs >= current || atomic.CompareAndSwapInt64(&s.stats.minLatency, current, durationNs) {
			break
		}
	}

	for {
		current := atomic.LoadInt64(&s.stats.maxLatency)
		if durationNs <= current || atomic.CompareAndSwapInt64(&s.stats.maxLatency, current, durationNs) {
			break
		}
	}
}

// Health and statistics

func (s *Searcher) IsHealthy() bool {
	healthy := s.getHealthyShards()
	return len(healthy) > 0
}

// IsRunning returns true if the searcher is currently running
func (s *Searcher) IsRunning() bool {
	return atomic.LoadInt32(&s.running) == 1
}

func (s *Searcher) GetStats() *Stats {
	s.stats.mutex.RLock()
	defer s.stats.mutex.RUnlock()

	stats := &Stats{
		TotalSearches:      atomic.LoadInt64(&s.stats.totalSearches),
		SuccessfulSearches: atomic.LoadInt64(&s.stats.successfulSearches),
		FailedSearches:     atomic.LoadInt64(&s.stats.failedSearches),
		ActiveSearches:     atomic.LoadInt32(&s.stats.activeSearches),
		QueuedSearches:     len(s.semaphore),
	}

	// Calculate average latency
	totalLatency := atomic.LoadInt64(&s.stats.totalLatency)
	if stats.TotalSearches > 0 {
		stats.AverageLatency = time.Duration(totalLatency / stats.TotalSearches)
	}

	stats.MinLatency = time.Duration(atomic.LoadInt64(&s.stats.minLatency))
	stats.MaxLatency = time.Duration(atomic.LoadInt64(&s.stats.maxLatency))

	// Copy shard stats
	stats.ShardStats = make([]*ShardSearchStats, 0, len(s.stats.shardStats))
	for _, stat := range s.stats.shardStats {
		statCopy := *stat
		stats.ShardStats = append(stats.ShardStats, &statCopy)
	}

	// Add cache stats if cache is enabled
	if s.cache != nil {
		stats.CacheStats = s.cache.GetStats()
	}

	return stats
}

func (s *Searcher) GetConfig() *Config {
	return s.config
}

// GetShards returns the underlying shards for cardinality counting
func (s *Searcher) GetShards() []bleve.Index {
	return s.shards
}

// SwapShards atomically replaces the current shards with new ones using IndexAlias.Swap()
// This follows Bleve best practices for zero-downtime index updates
func (s *Searcher) SwapShards(newShards []bleve.Index) error {
	if atomic.LoadInt32(&s.running) == 0 {
		return fmt.Errorf("searcher is not running")
	}

	if s.indexAlias == nil {
		return fmt.Errorf("indexAlias is nil")
	}

	newShards = wrapRecoveringIndexes(newShards)

	// Store old shards for logging
	oldShards := s.shards

	// Perform atomic swap using IndexAlias - this is the key Bleve operation
	// that provides zero-downtime index updates
	logger.Debugf("SwapShards: Starting atomic swap - old=%d, new=%d", len(oldShards), len(newShards))

	swapStartTime := time.Now()
	s.indexAlias.Swap(newShards, oldShards)
	swapDuration := time.Since(swapStartTime)

	logger.Infof("IndexAlias.Swap completed in %v (old=%d shards, new=%d shards)",
		swapDuration, len(oldShards), len(newShards))

	// Update internal shards reference to match the IndexAlias
	s.shards = newShards

	// Clear cache after shard swap to prevent stale results
	// Use goroutine to avoid potential deadlock during shard swap
	if s.cache != nil {
		// Capture cache reference to avoid race condition
		cache := s.cache
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
	s.stats.mutex.Lock()
	// Clear old shard stats
	s.stats.shardStats = make(map[int]*ShardSearchStats)
	// Initialize stats for new shards
	for i := range newShards {
		s.stats.shardStats[i] = &ShardSearchStats{
			ShardID:   i,
			IsHealthy: true,
		}
	}
	s.stats.mutex.Unlock()

	logger.Debugf("IndexAlias.Swap() completed: %d old shards -> %d new shards",
		len(oldShards), len(newShards))

	// Test the searcher with a simple query to verify functionality
	testCtx := context.Background()
	testReq := &SearchRequest{
		Limit:  1,
		Offset: 0,
	}

	if _, err := s.Search(testCtx, testReq); err != nil {
		logger.Errorf("Post-swap searcher test query failed: %v", err)
		return fmt.Errorf("searcher test failed after shard swap: %w", err)
	} else {
		logger.Debug("Post-swap searcher test query succeeded")
	}

	return nil
}

// Stop gracefully stops the searcher and closes all bleve indexes
func (s *Searcher) Stop() error {
	var err error

	s.closeOnce.Do(func() {
		// Set running to 0
		atomic.StoreInt32(&s.running, 0)

		// Close the index alias first (this doesn't close underlying indexes)
		if s.indexAlias != nil {
			if closeErr := s.indexAlias.Close(); closeErr != nil {
				logger.Errorf("Failed to close index alias: %v", closeErr)
				err = closeErr
			}
			s.indexAlias = nil
		}

		// DON'T close the underlying shards - they are managed by the indexer/shard manager
		// The searcher is just a consumer of these shards, not the owner
		// Clear the shards slice reference without closing the indexes
		s.shards = nil

		// Close cache if it exists
		if s.cache != nil {
			s.cache.Close()
			s.cache = nil
		}
	})

	return err
}
