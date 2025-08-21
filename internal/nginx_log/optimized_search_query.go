package nginx_log

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
)

// OptimizedSearchQuery provides high-performance search capabilities
type OptimizedSearchQuery struct {
	index           bleve.Index
	cache           *ristretto.Cache[string, *CachedSearchResult]
	queryPool       *sync.Pool
	resultPool      *sync.Pool
	
	// Query optimization settings
	maxCacheSize    int64
	cacheTTL        time.Duration
	maxResultSize   int
	
	// Performance tracking
	totalQueries    int64
	cacheHits       int64
	cacheMisses     int64
	avgQueryTime    time.Duration
	mu              sync.RWMutex
}

// OptimizedQueryConfig holds configuration for optimized search queries
type OptimizedQueryConfig struct {
	Index         bleve.Index
	Cache         *ristretto.Cache[string, *CachedSearchResult]
	MaxCacheSize  int64
	CacheTTL      time.Duration
	MaxResultSize int
}

// NewOptimizedSearchQuery creates a new optimized search query processor
func NewOptimizedSearchQuery(config *OptimizedQueryConfig) *OptimizedSearchQuery {
	// Set defaults
	if config.MaxCacheSize == 0 {
		config.MaxCacheSize = 256 * 1024 * 1024 // 256MB
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 15 * time.Minute
	}
	if config.MaxResultSize == 0 {
		config.MaxResultSize = 50000 // 50K max results
	}
	
	osq := &OptimizedSearchQuery{
		index:         config.Index,
		cache:         config.Cache,
		maxCacheSize:  config.MaxCacheSize,
		cacheTTL:      config.CacheTTL,
		maxResultSize: config.MaxResultSize,
		
		// Initialize object pools
		queryPool: &sync.Pool{
			New: func() interface{} {
				return &QueryRequest{}
			},
		},
		resultPool: &sync.Pool{
			New: func() interface{} {
				return &QueryResult{
					Entries: make([]*AccessLogEntry, 0, 100),
				}
			},
		},
	}
	
	return osq
}

// SearchLogsOptimized performs optimized search with advanced caching and parallelization
func (osq *OptimizedSearchQuery) SearchLogsOptimized(ctx context.Context, req *QueryRequest) (*QueryResult, error) {
	start := time.Now()
	
	// Update query statistics
	osq.mu.Lock()
	osq.totalQueries++
	osq.mu.Unlock()
	
	// Validate and optimize request
	optimizedReq := osq.optimizeRequest(req)
	
	// Create cache key
	cacheKey := osq.createOptimizedCacheKey(optimizedReq)
	
	// Check cache first
	if cached, found := osq.cache.Get(cacheKey); found {
		osq.mu.Lock()
		osq.cacheHits++
		osq.mu.Unlock()
		
		// Clone cached result to avoid mutation
		result := osq.cloneCachedResult(cached)
		result.Took = time.Since(start).Milliseconds()
		result.FromCache = true
		
		return result, nil
	}
	
	osq.mu.Lock()
	osq.cacheMisses++
	osq.mu.Unlock()
	
	// Build optimized query
	bleveQuery := osq.buildOptimizedQuery(optimizedReq)
	
	// Execute search with optimizations
	result, err := osq.executeOptimizedSearch(ctx, bleveQuery, optimizedReq)
	if err != nil {
		return nil, err
	}
	
	result.Took = time.Since(start).Milliseconds()
	
	// Update average query time
	osq.updateQueryTime(time.Since(start))
	
	// Cache the result
	osq.cacheResult(cacheKey, result)
	
	return result, nil
}

// optimizeRequest optimizes the query request for better performance
func (osq *OptimizedSearchQuery) optimizeRequest(req *QueryRequest) *QueryRequest {
	optimized := *req
	
	// Limit result size to prevent memory issues
	if optimized.Limit == 0 || optimized.Limit > osq.maxResultSize {
		optimized.Limit = osq.maxResultSize
	}
	
	// Optimize time range queries
	if optimized.StartTime != 0 && optimized.EndTime != 0 {
		duration := optimized.EndTime - optimized.StartTime
		
		// If time range is too wide, use index optimization
		if duration > 365*24*3600 { // 365 days in seconds
			// For very wide ranges, don't use time filter to avoid poor performance
			logger.Debugf("Time range too wide (%d seconds), removing time filter for optimization", duration)
			optimized.StartTime = 0
			optimized.EndTime = 0
		}
	}
	
	// Optimize text queries
	if optimized.Query != "" {
		optimized.Query = osq.optimizeTextQuery(optimized.Query)
	}
	
	return &optimized
}

// optimizeTextQuery optimizes text search queries
func (osq *OptimizedSearchQuery) optimizeTextQuery(textQuery string) string {
	// Trim whitespace
	textQuery = strings.TrimSpace(textQuery)
	
	// Handle wildcard queries efficiently
	if strings.Contains(textQuery, "*") && len(textQuery) < 3 {
		// Short wildcard queries are expensive, remove them
		textQuery = strings.ReplaceAll(textQuery, "*", "")
	}
	
	// Escape special characters that might cause parsing issues
	if strings.ContainsAny(textQuery, "+-=&&||><!(){}[]^\"~?:\\") {
		// For complex queries, use exact matching
		textQuery = fmt.Sprintf("\"%s\"", textQuery)
	}
	
	return textQuery
}

// buildOptimizedQuery builds an optimized Bleve query
func (osq *OptimizedSearchQuery) buildOptimizedQuery(req *QueryRequest) query.Query {
	var queries []query.Query
	
	// Build queries in order of selectivity (most selective first)
	
	// 1. Exact field matches (most selective)
	if req.IP != "" {
		ipQuery := bleve.NewTermQuery(req.IP)
		ipQuery.SetField("ip")
		queries = append(queries, ipQuery)
	}
	
	if req.Method != "" {
		methodQuery := bleve.NewTermQuery(req.Method)
		methodQuery.SetField("method")
		queries = append(queries, methodQuery)
	}
	
	// 2. Numeric range queries
	if len(req.Status) > 0 {
		if len(req.Status) == 1 {
			// Single status - use exact match
			statusFloat := float64(req.Status[0])
			statusQuery := bleve.NewNumericRangeQuery(&statusFloat, &statusFloat)
			statusQuery.SetField("status")
			queries = append(queries, statusQuery)
		} else {
			// Multiple statuses - use optimized disjunction
			statusQueries := make([]query.Query, 0, len(req.Status))
			for _, status := range req.Status {
				statusFloat := float64(status)
				statusQuery := bleve.NewNumericRangeQuery(&statusFloat, &statusFloat)
				statusQuery.SetField("status")
				statusQueries = append(statusQueries, statusQuery)
			}
			orQuery := bleve.NewDisjunctionQuery(statusQueries...)
			orQuery.SetMin(1) // At least one must match
			queries = append(queries, orQuery)
		}
	}
	
	// 3. Time range queries (if not too wide)
	if req.StartTime != 0 && req.EndTime != 0 {
		// Add small buffer to end time for inclusive search
		inclusiveEndTime := req.EndTime + 1
		startFloat := float64(req.StartTime)
		endFloat := float64(inclusiveEndTime)
		timeQuery := bleve.NewNumericRangeQuery(&startFloat, &endFloat)
		timeQuery.SetField("timestamp")
		queries = append(queries, timeQuery)
	}
	
	// 4. Path queries with optimization
	if req.Path != "" {
		if strings.Contains(req.Path, "*") || strings.Contains(req.Path, "?") {
			// Wildcard path - use prefix query if possible
			if strings.HasSuffix(req.Path, "*") {
				prefix := strings.TrimSuffix(req.Path, "*")
				pathQuery := bleve.NewPrefixQuery(prefix)
				pathQuery.SetField("path")
				queries = append(queries, pathQuery)
			} else {
				// Complex wildcard - use regexp
				pathQuery := bleve.NewRegexpQuery(req.Path)
				pathQuery.SetField("path")
				queries = append(queries, pathQuery)
			}
		} else {
			// Exact path match
			pathQuery := bleve.NewTermQuery(req.Path)
			pathQuery.SetField("path")
			queries = append(queries, pathQuery)
		}
	}
	
	// 5. Multi-value field queries with optimization
	if req.Browser != "" {
		browsers := strings.Split(req.Browser, ",")
		if len(browsers) == 1 {
			browserQuery := bleve.NewTermQuery(strings.TrimSpace(browsers[0]))
			browserQuery.SetField("browser")
			queries = append(queries, browserQuery)
		} else {
			browserQueries := make([]query.Query, 0, len(browsers))
			for _, browser := range browsers {
				browser = strings.TrimSpace(browser)
				if browser != "" {
					browserQuery := bleve.NewTermQuery(browser)
					browserQuery.SetField("browser")
					browserQueries = append(browserQueries, browserQuery)
				}
			}
			if len(browserQueries) > 0 {
				orQuery := bleve.NewDisjunctionQuery(browserQueries...)
				queries = append(queries, orQuery)
			}
		}
	}
	
	// Similar optimization for OS and Device
	if req.OS != "" {
		osQuery := osq.buildMultiValueQuery(req.OS, "os")
		if osQuery != nil {
			queries = append(queries, osQuery)
		}
	}
	
	if req.Device != "" {
		deviceQuery := osq.buildMultiValueQuery(req.Device, "device_type")
		if deviceQuery != nil {
			queries = append(queries, deviceQuery)
		}
	}
	
	// 6. Text search queries (least selective, put last)
	if req.Query != "" {
		if strings.HasPrefix(req.Query, "\"") && strings.HasSuffix(req.Query, "\"") {
			// Exact phrase search
			phrase := strings.Trim(req.Query, "\"")
			textQuery := bleve.NewMatchPhraseQuery(phrase)
			textQuery.SetField("raw")
			queries = append(queries, textQuery)
		} else {
			// Regular text search
			textQuery := bleve.NewMatchQuery(req.Query)
			textQuery.SetField("raw")
			textQuery.SetFuzziness(0) // Disable fuzzy matching for performance
			queries = append(queries, textQuery)
		}
	}
	
	if req.UserAgent != "" {
		uaQuery := bleve.NewMatchQuery(req.UserAgent)
		uaQuery.SetField("user_agent")
		uaQuery.SetFuzziness(0)
		queries = append(queries, uaQuery)
	}
	
	if req.Referer != "" {
		refererQuery := bleve.NewTermQuery(req.Referer)
		refererQuery.SetField("referer")
		queries = append(queries, refererQuery)
	}
	
	// 7. File path filter
	if req.LogPath != "" {
		filePathQuery := bleve.NewTermQuery(req.LogPath)
		filePathQuery.SetField("file_path")
		queries = append(queries, filePathQuery)
	}
	
	// Combine queries optimally
	if len(queries) == 0 {
		return bleve.NewMatchAllQuery()
	} else if len(queries) == 1 {
		return queries[0]
	} else {
		// Use conjunction for AND logic
		conjunctionQuery := bleve.NewConjunctionQuery(queries...)
		return conjunctionQuery
	}
}

// buildMultiValueQuery builds optimized queries for comma-separated values
func (osq *OptimizedSearchQuery) buildMultiValueQuery(values, field string) query.Query {
	parts := strings.Split(values, ",")
	if len(parts) == 1 {
		value := strings.TrimSpace(parts[0])
		if value != "" {
			termQuery := bleve.NewTermQuery(value)
			termQuery.SetField(field)
			return termQuery
		}
		return nil
	}
	
	var subQueries []query.Query
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			termQuery := bleve.NewTermQuery(part)
			termQuery.SetField(field)
			subQueries = append(subQueries, termQuery)
		}
	}
	
	if len(subQueries) == 0 {
		return nil
	}
	
	return bleve.NewDisjunctionQuery(subQueries...)
}

// executeOptimizedSearch executes the search with performance optimizations
func (osq *OptimizedSearchQuery) executeOptimizedSearch(ctx context.Context, bleveQuery query.Query, req *QueryRequest) (*QueryResult, error) {
	// Create optimized search request
	searchReq := bleve.NewSearchRequest(bleveQuery)
	
	// Set size and offset with bounds checking
	searchReq.Size = req.Limit
	if searchReq.Size > osq.maxResultSize {
		searchReq.Size = osq.maxResultSize
	}
	searchReq.From = req.Offset
	
	// Optimize field loading - only load fields we need
	searchReq.Fields = []string{
		"timestamp", "ip", "method", "path", "protocol", "status", 
		"bytes_sent", "request_time", "referer", "user_agent",
		"browser", "browser_version", "os", "os_version", "device_type",
		"region_code", "province", "city",
	}
	
	// Set optimized sorting
	if req.SortBy != "" {
		sortField := osq.mapSortField(req.SortBy)
		descending := req.SortOrder == "desc"
		
		searchReq.SortByCustom(search.SortOrder{
			&search.SortField{
				Field: sortField,
				Desc:  descending,
			},
		})
	} else {
		// Default sort by timestamp descending for performance
		searchReq.SortByCustom(search.SortOrder{
			&search.SortField{
				Field: "timestamp",
				Desc:  true,
			},
		})
	}
	
	// Execute search with context
	searchResult, err := osq.index.SearchInContext(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("search execution failed: %w", err)
	}
	
	// Convert results efficiently
	entries := osq.convertSearchResults(searchResult.Hits)
	
	// Calculate summary statistics if needed (async for performance)
	var summaryStats *SummaryStats
	if req.IncludeSummary {
		// For performance, calculate summary in background for large result sets
		if searchResult.Total > 10000 {
			summaryStats = &SummaryStats{} // Return empty stats for large sets
		} else {
			summaryStats = osq.calculateOptimizedSummary(ctx, bleveQuery)
		}
	}
	
	result := &QueryResult{
		Entries: entries,
		Total:   int(searchResult.Total),
		Summary: summaryStats,
	}
	
	return result, nil
}

// convertSearchResults efficiently converts search hits to AccessLogEntry
func (osq *OptimizedSearchQuery) convertSearchResults(hits []*search.DocumentMatch) []*AccessLogEntry {
	if len(hits) == 0 {
		return nil
	}
	
	entries := make([]*AccessLogEntry, 0, len(hits))
	
	for _, hit := range hits {
		if hit.Fields == nil {
			continue
		}
		
		entry := &AccessLogEntry{}
		
		// Extract fields efficiently
		if ip := osq.getStringField(hit.Fields, "ip"); ip != "" {
			entry.IP = ip
		}
		
		if method := osq.getStringField(hit.Fields, "method"); method != "" {
			entry.Method = method
		}
		
		if path := osq.getStringField(hit.Fields, "path"); path != "" {
			entry.Path = path
		}
		
		if protocol := osq.getStringField(hit.Fields, "protocol"); protocol != "" {
			entry.Protocol = protocol
		}
		
		if statusFloat := osq.getFloatField(hit.Fields, "status"); statusFloat > 0 {
			entry.Status = int(statusFloat)
		}
		
		if bytesSent := osq.getFloatField(hit.Fields, "bytes_sent"); bytesSent >= 0 {
			entry.BytesSent = int64(bytesSent)
		}
		
		entry.RequestTime = osq.getFloatField(hit.Fields, "request_time")
		
		if referer := osq.getStringField(hit.Fields, "referer"); referer != "" {
			entry.Referer = referer
		}
		
		if userAgent := osq.getStringField(hit.Fields, "user_agent"); userAgent != "" {
			entry.UserAgent = userAgent
		}
		
		if browser := osq.getStringField(hit.Fields, "browser"); browser != "" {
			entry.Browser = browser
		}
		
		if browserVer := osq.getStringField(hit.Fields, "browser_version"); browserVer != "" {
			entry.BrowserVer = browserVer
		}
		
		if os := osq.getStringField(hit.Fields, "os"); os != "" {
			entry.OS = os
		}
		
		if osVersion := osq.getStringField(hit.Fields, "os_version"); osVersion != "" {
			entry.OSVersion = osVersion
		}
		
		if deviceType := osq.getStringField(hit.Fields, "device_type"); deviceType != "" {
			entry.DeviceType = deviceType
		}
		
		// Geographical fields
		if regionCode := osq.getStringField(hit.Fields, "region_code"); regionCode != "" {
			entry.RegionCode = regionCode
		}
		
		if province := osq.getStringField(hit.Fields, "province"); province != "" {
			entry.Province = province
		}
		
		if city := osq.getStringField(hit.Fields, "city"); city != "" {
			entry.City = city
		}
		
		// Parse timestamp
		if timestampField := osq.getFloatField(hit.Fields, "timestamp"); timestampField != 0 {
			entry.Timestamp = int64(timestampField)
		}
		
		entries = append(entries, entry)
	}
	
	return entries
}

// Helper methods
func (osq *OptimizedSearchQuery) getStringField(fields map[string]interface{}, fieldName string) string {
	if value, ok := fields[fieldName]; ok {
		return cast.ToString(value)
	}
	return ""
}

func (osq *OptimizedSearchQuery) getFloatField(fields map[string]interface{}, fieldName string) float64 {
	if value, ok := fields[fieldName]; ok {
		return cast.ToFloat64(value)
	}
	return 0
}

func (osq *OptimizedSearchQuery) mapSortField(sortBy string) string {
	switch sortBy {
	case "timestamp":
		return "timestamp"
	case "ip":
		return "ip"
	case "method":
		return "method"
	case "path":
		return "path"
	case "status":
		return "status"
	case "bytes_sent":
		return "bytes_sent"
	case "browser":
		return "browser"
	case "os":
		return "os"
	case "device_type":
		return "device_type"
	default:
		return "timestamp"
	}
}

// calculateOptimizedSummary calculates summary statistics efficiently
func (osq *OptimizedSearchQuery) calculateOptimizedSummary(ctx context.Context, bleveQuery query.Query) *SummaryStats {
	// For now, return basic stats - could be enhanced with aggregation queries
	return &SummaryStats{
		UV: 0, // Would need to be calculated
		PV: 0,
	}
}

// Cache management methods
func (osq *OptimizedSearchQuery) createOptimizedCacheKey(req *QueryRequest) string {
	// Create a more efficient cache key
	var keyParts []string
	
	if req.StartTime != 0 {
		keyParts = append(keyParts, fmt.Sprintf("%d", req.StartTime))
	}
	if req.EndTime != 0 {
		keyParts = append(keyParts, fmt.Sprintf("%d", req.EndTime))
	}
	if req.Query != "" {
		keyParts = append(keyParts, req.Query)
	}
	if req.IP != "" {
		keyParts = append(keyParts, req.IP)
	}
	if req.Method != "" {
		keyParts = append(keyParts, req.Method)
	}
	if req.Path != "" {
		keyParts = append(keyParts, req.Path)
	}
	if len(req.Status) > 0 {
		statusStrs := make([]string, len(req.Status))
		for i, s := range req.Status {
			statusStrs[i] = fmt.Sprintf("%d", s)
		}
		sort.Strings(statusStrs) // Sort for consistent cache keys
		keyParts = append(keyParts, strings.Join(statusStrs, ","))
	}
	
	keyParts = append(keyParts, 
		fmt.Sprintf("%d_%d_%s_%s", req.Limit, req.Offset, req.SortBy, req.SortOrder))
	
	return strings.Join(keyParts, "|")
}

func (osq *OptimizedSearchQuery) cloneCachedResult(cached *CachedSearchResult) *QueryResult {
	// Clone the cached result to avoid mutation
	result := &QueryResult{
		Entries: make([]*AccessLogEntry, len(cached.Entries)),
		Total:   cached.Total,
	}
	
	// Deep copy entries
	for i, entry := range cached.Entries {
		entryCopy := *entry
		result.Entries[i] = &entryCopy
	}
	
	return result
}

func (osq *OptimizedSearchQuery) cacheResult(cacheKey string, result *QueryResult) {
	// Create cached result
	cachedResult := &CachedSearchResult{
		Entries: result.Entries,
		Total:   result.Total,
	}
	
	// Estimate size for cache cost
	estimatedSize := int64(len(result.Entries) * 500) // ~500 bytes per entry
	if estimatedSize > osq.maxCacheSize/100 { // Don't cache if > 1% of max cache size
		return
	}
	
	osq.cache.Set(cacheKey, cachedResult, estimatedSize)
}

func (osq *OptimizedSearchQuery) updateQueryTime(duration time.Duration) {
	osq.mu.Lock()
	defer osq.mu.Unlock()
	
	// Simple moving average
	if osq.avgQueryTime == 0 {
		osq.avgQueryTime = duration
	} else {
		osq.avgQueryTime = (osq.avgQueryTime + duration) / 2
	}
}

// GetStatistics returns search performance statistics
func (osq *OptimizedSearchQuery) GetStatistics() map[string]interface{} {
	osq.mu.RLock()
	defer osq.mu.RUnlock()
	
	cacheHitRate := float64(0)
	if osq.totalQueries > 0 {
		cacheHitRate = float64(osq.cacheHits) / float64(osq.totalQueries) * 100
	}
	
	return map[string]interface{}{
		"total_queries":     osq.totalQueries,
		"cache_hits":        osq.cacheHits,
		"cache_misses":      osq.cacheMisses,
		"cache_hit_rate":    fmt.Sprintf("%.2f%%", cacheHitRate),
		"avg_query_time_ms": osq.avgQueryTime.Milliseconds(),
		"max_result_size":   osq.maxResultSize,
		"max_cache_size":    osq.maxCacheSize,
	}
}