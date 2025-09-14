package searcher

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/uozi-tech/cosy/logger"
)

// CardinalityCounter provides efficient unique value counting without large FacetSize
type CardinalityCounter struct {
	indexAlias bleve.IndexAlias // Use IndexAlias instead of individual shards
	shards     []bleve.Index    // Keep shards for fallback if needed
	mu         sync.RWMutex
	stopOnce   sync.Once
}

// NewCardinalityCounter creates a new cardinality counter
func NewCardinalityCounter(shards []bleve.Index) *CardinalityCounter {
	var indexAlias bleve.IndexAlias
	if len(shards) > 0 {
		// Create IndexAlias for distributed search like DistributedSearcher does
		indexAlias = bleve.NewIndexAlias(shards...)

		// Note: IndexAlias doesn't have SetIndexMapping method
		// The mapping will be inherited from the constituent indices
		logger.Debugf("Created IndexAlias for cardinality counter with %d shards", len(shards))
	}

	return &CardinalityCounter{
		indexAlias: indexAlias,
		shards:     shards,
	}
}

// Stop gracefully closes the counter's resources, like the IndexAlias.
func (cc *CardinalityCounter) Stop() error {
	var err error
	cc.stopOnce.Do(func() {
		cc.mu.Lock()
		defer cc.mu.Unlock()

		if cc.indexAlias != nil {
			logger.Debugf("Closing IndexAlias in CardinalityCounter")
			err = cc.indexAlias.Close()
			cc.indexAlias = nil
		}
		cc.shards = nil
	})
	return err
}

// CardinalityRequest represents a request for unique value counting
type CardinalityRequest struct {
	Field          string      `json:"field"`
	Query          query.Query `json:"query,omitempty"` // Optional query to filter documents
	StartTime      *int64      `json:"start_time,omitempty"`
	EndTime        *int64      `json:"end_time,omitempty"`
	LogPaths       []string    `json:"log_paths,omitempty"`
	UseMainLogPath bool        `json:"use_main_log_path,omitempty"` // Use main_log_path field instead of file_path
}

// CardinalityResult represents the result of cardinality counting
type CardinalityResult struct {
	Field       string `json:"field"`
	Cardinality uint64 `json:"cardinality"`
	TotalDocs   uint64 `json:"total_docs"`
	Error       string `json:"error,omitempty"`
}

// CountCardinality efficiently counts unique values using IndexAlias with global scoring
// This leverages Bleve's distributed search optimizations and avoids FacetSize limits
func (cc *CardinalityCounter) CountCardinality(ctx context.Context, req *CardinalityRequest) (*CardinalityResult, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	if req.Field == "" {
		return nil, fmt.Errorf("field name is required")
	}

	if cc.indexAlias == nil {
		return &CardinalityResult{
			Field: req.Field,
			Error: "IndexAlias not available",
		}, fmt.Errorf("IndexAlias not available")
	}

	// Use IndexAlias with global scoring for consistent distributed search
	uniqueTerms, totalDocs, err := cc.collectTermsUsingIndexAlias(ctx, req)
	if err != nil {
		return &CardinalityResult{
			Field: req.Field,
			Error: fmt.Sprintf("failed to collect terms: %v", err),
		}, err
	}

	logger.Infof("Cardinality count completed: field='%s', unique_terms=%d, total_docs=%d",
		req.Field, len(uniqueTerms), totalDocs)

	return &CardinalityResult{
		Field:       req.Field,
		Cardinality: uint64(len(uniqueTerms)),
		TotalDocs:   totalDocs,
	}, nil
}

// collectTermsUsingIndexAlias collects unique terms using IndexAlias with global scoring
func (cc *CardinalityCounter) collectTermsUsingIndexAlias(ctx context.Context, req *CardinalityRequest) (map[string]struct{}, uint64, error) {
	uniqueTerms := make(map[string]struct{})

	// Enable global scoring context like DistributedSearcher does
	globalCtx := context.WithValue(ctx, search.SearchTypeKey, search.GlobalScoring)

	// Strategy 1: Try large facet first (more efficient for most cases)
	terms1, totalDocs, err1 := cc.collectTermsUsingLargeFacet(globalCtx, req)
	if err1 != nil {
		logger.Warnf("Large facet collection failed: %v", err1)
	} else {
		for term := range terms1 {
			uniqueTerms[term] = struct{}{}
		}
		logger.Infof("Large facet collected %d unique terms", len(terms1))
	}

	// Strategy 2: Use pagination if facet was likely truncated or failed
	needsPagination := len(terms1) >= 50000 || err1 != nil
	if needsPagination {
		logger.Infof("Using pagination to collect remaining terms...")
		terms2, _, err2 := cc.collectTermsUsingPagination(globalCtx, req)
		if err2 != nil {
			logger.Warnf("Pagination collection failed: %v", err2)
		} else {
			for term := range terms2 {
				uniqueTerms[term] = struct{}{}
			}
			logger.Infof("Pagination collected additional terms, total unique: %d", len(uniqueTerms))
		}
	}

	return uniqueTerms, totalDocs, nil
}

// collectTermsUsingLargeFacet uses IndexAlias with a large facet to efficiently collect terms
func (cc *CardinalityCounter) collectTermsUsingLargeFacet(ctx context.Context, req *CardinalityRequest) (map[string]struct{}, uint64, error) {
	terms := make(map[string]struct{})

	// Build search request using IndexAlias with proper filtering
	boolQuery := bleve.NewBooleanQuery()
	boolQuery.AddMust(bleve.NewMatchAllQuery())

	// Add time range filter if specified
	if req.StartTime != nil && req.EndTime != nil {
		startTime := float64(*req.StartTime)
		endTime := float64(*req.EndTime)
		timeQuery := bleve.NewNumericRangeQuery(&startTime, &endTime)
		timeQuery.SetField("timestamp")
		boolQuery.AddMust(timeQuery)
	}

	// Add log path filters - use main_log_path or file_path based on request
	if len(req.LogPaths) > 0 {
		logPathQuery := bleve.NewBooleanQuery()
		fieldName := "file_path" // default
		if req.UseMainLogPath {
			fieldName = "main_log_path"
		}
		for _, logPath := range req.LogPaths {
			termQuery := bleve.NewTermQuery(logPath)
			termQuery.SetField(fieldName)
			logPathQuery.AddShould(termQuery)
		}
		logPathQuery.SetMinShould(1)
		boolQuery.AddMust(logPathQuery)
	}

	searchReq := bleve.NewSearchRequest(boolQuery)
	searchReq.Size = 0 // We don't need documents, just facets

	// Use very large facet size - we're back to this approach but using IndexAlias
	// which should handle it more efficiently than individual shards
	facetSize := 100000 // Large size for maximum accuracy
	facet := bleve.NewFacetRequest(req.Field, facetSize)
	searchReq.AddFacet(req.Field, facet)

	// Debug: Log the constructed query
	if queryBytes, err := json.Marshal(searchReq.Query); err == nil {
		logger.Debugf("CardinalityCounter query: %s", string(queryBytes))
	}

	// Execute search using IndexAlias with global scoring context
	result, err := cc.indexAlias.SearchInContext(ctx, searchReq)
	if err != nil {
		return terms, 0, fmt.Errorf("IndexAlias facet search failed: %w", err)
	}

	logger.Debugf("CardinalityCounter facet search result: Total=%d, Facets=%v", result.Total, result.Facets != nil)

	// Extract terms from facet result
	if facetResult, ok := result.Facets[req.Field]; ok && facetResult.Terms != nil {
		facetTerms := facetResult.Terms.Terms()
		for _, term := range facetTerms {
			terms[term.Term] = struct{}{}
		}

		logger.Infof("IndexAlias large facet: collected %d terms, facet.Total=%d, result.Total=%d",
			len(terms), facetResult.Total, result.Total)

		// Check if facet was truncated
		if len(facetTerms) >= facetSize {
			logger.Warnf("Facet likely truncated at %d terms, total unique may be higher", facetSize)
		}
	}

	return terms, result.Total, nil
}

// collectTermsUsingPagination uses IndexAlias with pagination to collect all terms
func (cc *CardinalityCounter) collectTermsUsingPagination(ctx context.Context, req *CardinalityRequest) (map[string]struct{}, uint64, error) {
	terms := make(map[string]struct{})

	pageSize := 10000 // Large page size for efficiency
	maxPages := 1000  // Support very large datasets
	processedDocs := 0

	logger.Infof("Starting IndexAlias pagination for field '%s' (pageSize=%d)", req.Field, pageSize)

	for page := 0; page < maxPages; page++ {
		// Build proper query with all filters
		boolQuery := bleve.NewBooleanQuery()
		boolQuery.AddMust(bleve.NewMatchAllQuery())

		// Add time range filter if specified
		if req.StartTime != nil && req.EndTime != nil {
			startTime := float64(*req.StartTime)
			endTime := float64(*req.EndTime)
			timeQuery := bleve.NewNumericRangeQuery(&startTime, &endTime)
			timeQuery.SetField("timestamp")
			boolQuery.AddMust(timeQuery)
		}

		// Add log path filters, respecting UseMainLogPath
		if len(req.LogPaths) > 0 {
			logPathQuery := bleve.NewBooleanQuery()
			fieldName := "file_path"
			if req.UseMainLogPath {
				fieldName = "main_log_path"
			}
			for _, logPath := range req.LogPaths {
				termQuery := bleve.NewTermQuery(logPath)
				termQuery.SetField(fieldName)
				logPathQuery.AddShould(termQuery)
			}
			logPathQuery.SetMinShould(1)
			boolQuery.AddMust(logPathQuery)
		}

		searchReq := bleve.NewSearchRequest(boolQuery)
		searchReq.Size = pageSize
		searchReq.From = page * pageSize
		searchReq.Fields = []string{req.Field}

		// Execute with IndexAlias and global scoring
		result, err := cc.indexAlias.SearchInContext(ctx, searchReq)
		if err != nil {
			return terms, 0, fmt.Errorf("IndexAlias pagination search failed at page %d: %w", page, err)
		}

		// If no more hits, we're done
		if len(result.Hits) == 0 {
			logger.Infof("Pagination complete: processed %d documents, found %d unique terms",
				processedDocs, len(terms))
			break
		}

		// Extract unique terms from documents
		for _, hit := range result.Hits {
			if fieldValue, ok := hit.Fields[req.Field]; ok {
				if strValue, ok := fieldValue.(string); ok && strValue != "" {
					terms[strValue] = struct{}{}
				}
			}
		}

		processedDocs += len(result.Hits)

		// Progress logging
		if processedDocs%50000 == 0 && processedDocs > 0 {
			logger.Infof("Pagination progress: processed %d documents, found %d unique terms",
				processedDocs, len(terms))
		}

		// If we got fewer results than pageSize, we've reached the end
		if len(result.Hits) < pageSize {
			logger.Infof("Pagination complete: processed %d documents, found %d unique terms",
				processedDocs, len(terms))
			break
		}

		// Generous safety limit
		if len(terms) > 500000 {
			logger.Warnf("Very large cardinality detected (%d terms), stopping for memory safety", len(terms))
			break
		}
	}

	return terms, uint64(processedDocs), nil
}

// EstimateCardinality provides a fast cardinality estimate using sampling approach
// This is useful for very large datasets where exact counting might be expensive
func (cc *CardinalityCounter) EstimateCardinality(ctx context.Context, req *CardinalityRequest) (*CardinalityResult, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	if req.Field == "" {
		return nil, fmt.Errorf("field name is required")
	}

	// Use statistical sampling for very large datasets
	// Take a sample and extrapolate to estimate total cardinality
	sampleSize := 10000 // Sample 10K documents
	uniqueInSample := make(map[string]struct{})
	totalSampleDocs := uint64(0)

	// Process each shard with sampling
	for i, shard := range cc.shards {
		if shard == nil {
			continue
		}

		shardSample, shardTotal, err := cc.sampleShardTerms(ctx, shard, req, sampleSize/len(cc.shards))
		if err != nil {
			logger.Errorf("Failed to sample shard %d: %v", i, err)
			continue
		}

		totalSampleDocs += shardTotal

		// Merge unique terms from sample
		for term := range shardSample {
			uniqueInSample[term] = struct{}{}
		}
	}

	// For now, still use exact counting for accuracy
	// In the future, we could use the sample to extrapolate:
	// sampledUnique := uint64(len(uniqueInSample))
	// estimatedCardinality := sampledUnique * (totalDocs / totalSampleDocs)

	if totalSampleDocs == 0 {
		return &CardinalityResult{
			Field:       req.Field,
			Cardinality: 0,
			TotalDocs:   0,
		}, nil
	}

	// For accurate results with large datasets, we use exact counting
	// The sampling code above is kept for future statistical estimation
	return cc.CountCardinality(ctx, req)
}

// sampleShardTerms takes a statistical sample from a shard for cardinality estimation
func (cc *CardinalityCounter) sampleShardTerms(ctx context.Context, shard bleve.Index, req *CardinalityRequest, sampleSize int) (map[string]struct{}, uint64, error) {
	terms := make(map[string]struct{})

	searchReq := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
	searchReq.Size = sampleSize
	searchReq.Fields = []string{req.Field}

	// Add time range filter if specified
	if req.StartTime != nil && req.EndTime != nil {
		startTime := float64(*req.StartTime)
		endTime := float64(*req.EndTime)
		timeQuery := bleve.NewNumericRangeQuery(&startTime, &endTime)
		timeQuery.SetField("timestamp")

		boolQuery := bleve.NewBooleanQuery()
		boolQuery.AddMust(searchReq.Query)
		boolQuery.AddMust(timeQuery)
		searchReq.Query = boolQuery
	}

	result, err := shard.Search(searchReq)
	if err != nil {
		return terms, 0, err
	}

	// Extract terms from sample
	for _, hit := range result.Hits {
		if fieldValue, ok := hit.Fields[req.Field]; ok {
			if strValue, ok := fieldValue.(string); ok && strValue != "" {
				terms[strValue] = struct{}{}
			}
		}
	}

	return terms, result.Total, nil
}

// BatchCountCardinality counts cardinality for multiple fields efficiently
func (cc *CardinalityCounter) BatchCountCardinality(ctx context.Context, fields []string, baseReq *CardinalityRequest) (map[string]*CardinalityResult, error) {
	results := make(map[string]*CardinalityResult)

	// Process fields in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, field := range fields {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()

			req := *baseReq // Copy base request
			req.Field = f

			result, err := cc.CountCardinality(ctx, &req)
			if err != nil {
				result = &CardinalityResult{
					Field: f,
					Error: err.Error(),
				}
			}

			mu.Lock()
			results[f] = result
			mu.Unlock()
		}(field)
	}

	wg.Wait()
	return results, nil
}
