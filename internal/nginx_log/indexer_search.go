package nginx_log

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"
)

// SearchLogs searches for log entries matching the given criteria
func (li *LogIndexer) SearchLogs(ctx context.Context, req *QueryRequest) (*QueryResult, error) {
	start := time.Now()

	// Create cache key
	cacheKey := li.createCacheKey(req)

	// Check cache first
	if cached, found := li.cache.Get(cacheKey); found {
		// Calculate summary statistics from cache (we still need to do this since cache doesn't store summary)
		summaryStats, err := li.calculateSummaryStatsFromQuery(ctx, li.buildSearchQuery(req))
		if err != nil {
			logger.Warnf("Failed to calculate summary statistics from cache: %v", err)
			summaryStats = &SummaryStats{}
		}

		return &QueryResult{
			Entries: cached.Entries,
			Total:   cached.Total,
			Took:    time.Since(start).Milliseconds(),
			Summary: summaryStats,
		}, nil
	}

	// Build search query
	query := li.buildSearchQuery(req)

	// Create search request
	searchReq := bleve.NewSearchRequest(query)
	// Handle unlimited search (Limit = 0)
	if req.Limit == 0 {
		searchReq.Size = 10000000 // Very large number for unlimited search
	} else {
		searchReq.Size = req.Limit
	}
	searchReq.From = req.Offset

	// Set sorting
	if req.SortBy != "" {
		sortField := li.mapSortField(req.SortBy)
		ascending := req.SortOrder == "asc"
		searchReq.SortBy([]string{sortField})
		if !ascending {
			// For descending sort, we need to use negative sorting
			// This is a workaround for Bleve v2
			searchReq.SortByCustom(search.SortOrder{
				&search.SortField{
					Field: sortField,
					Desc:  true,
				},
			})
		} else {
			searchReq.SortByCustom(search.SortOrder{
				&search.SortField{
					Field: sortField,
					Desc:  false,
				},
			})
		}
		logger.Infof("Applying sort: field=%s, order=%s (desc=%v)", sortField, req.SortOrder, !ascending)
	} else {
		// Default sort by timestamp descending
		searchReq.SortByCustom(search.SortOrder{
			&search.SortField{
				Field: "timestamp",
				Desc:  true,
			},
		})
	}

	// Include all fields in results
	searchReq.Fields = []string{"*"}


	// Execute search
	searchResult, err := li.index.SearchInContext(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Convert search results to AccessLogEntry
	entries := make([]*AccessLogEntry, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		entry := li.convertHitToEntry(hit)
		if entry != nil {
			entries = append(entries, entry)
		}
	}

	// Calculate summary statistics from ALL matching results (not just current page)
	summaryStats, err := li.calculateSummaryStatsFromQuery(ctx, query)
	if err != nil {
		logger.Warnf("Failed to calculate summary statistics: %v", err)
		summaryStats = &SummaryStats{} // Return empty stats on error
	}

	// Cache results with total count
	cachedResult := &CachedSearchResult{
		Entries: entries,
		Total:   int(searchResult.Total),
	}
	li.cache.Set(cacheKey, cachedResult, int64(len(entries)*500+100)) // Estimate 500 bytes per entry + overhead

	result := &QueryResult{
		Entries: entries,
		Total:   int(searchResult.Total),
		Took:    time.Since(start).Milliseconds(),
		Summary: summaryStats,
	}

	return result, nil
}

// buildSearchQuery builds a Bleve query based on the request parameters
func (li *LogIndexer) buildSearchQuery(req *QueryRequest) query.Query {
	var queries []query.Query

	// Time range query - only add if we have meaningful time constraints
	if req.StartTime != 0 && req.EndTime != 0 {
		// Check if the time range is reasonable (not too wide)
		duration := req.EndTime - req.StartTime
		if duration < 400*24*3600 { // Less than ~400 days in seconds
			// Add 1 second to endTime to ensure boundary values are included
			// This fixes the issue where records with exact endTime are excluded due to exclusive upper bound
			inclusiveEndTime := req.EndTime + 1
			logger.Infof("Using time range filter: %d to %d (inclusive)", req.StartTime, inclusiveEndTime)
			startFloat := float64(req.StartTime)
			endFloat := float64(inclusiveEndTime)
			timeQuery := bleve.NewNumericRangeQuery(&startFloat, &endFloat)
			timeQuery.SetField("timestamp")
			queries = append(queries, timeQuery)
		} else {
			logger.Infof("Time range too wide (%d seconds), ignoring time filter to search all data", duration)
		}
	} else {
		logger.Infof("No meaningful time range specified, searching all data")
	}

	// Text search query
	if req.Query != "" {
		textQuery := bleve.NewMatchQuery(req.Query)
		textQuery.SetField("raw")
		queries = append(queries, textQuery)
	}

	// IP filter
	if req.IP != "" {
		ipQuery := bleve.NewMatchQuery(req.IP)
		ipQuery.SetField("ip")
		queries = append(queries, ipQuery)
	}

	// Method filter
	if req.Method != "" {
		logger.Infof("Adding method filter: %s", req.Method)
		methodQuery := bleve.NewMatchQuery(req.Method)
		methodQuery.SetField("method")
		queries = append(queries, methodQuery)
	}

	// Status filter
	if len(req.Status) > 0 {
		logger.Infof("Adding status filter: %v", req.Status)
		var statusQueries []query.Query
		for _, status := range req.Status {
			// Use NumericRangeQuery for exact numeric match
			statusFloat := float64(status)
			statusQuery := bleve.NewNumericRangeQuery(&statusFloat, &statusFloat)
			statusQuery.SetField("status")
			statusQueries = append(statusQueries, statusQuery)
		}
		if len(statusQueries) == 1 {
			queries = append(queries, statusQueries[0])
		} else {
			orQuery := bleve.NewDisjunctionQuery(statusQueries...)
			queries = append(queries, orQuery)
		}
	}

	// Path filter
	if req.Path != "" {
		logger.Infof("Adding path filter: %s", req.Path)
		pathQuery := bleve.NewMatchQuery(req.Path)
		pathQuery.SetField("path")
		queries = append(queries, pathQuery)
	}

	// User agent filter
	if req.UserAgent != "" {
		uaQuery := bleve.NewMatchQuery(req.UserAgent)
		uaQuery.SetField("user_agent")
		queries = append(queries, uaQuery)
	}

	// Referer filter
	if req.Referer != "" {
		logger.Infof("Adding referer filter: %s", req.Referer)
		refererQuery := bleve.NewTermQuery(req.Referer)
		refererQuery.SetField("referer")
		queries = append(queries, refererQuery)
	}

	// Browser filter
	if req.Browser != "" {
		logger.Infof("Adding browser filter: %s", req.Browser)
		browsers := strings.Split(req.Browser, ",")
		var browserQueries []query.Query
		for _, browser := range browsers {
			browser = strings.TrimSpace(browser)
			if browser != "" {
				browserQuery := bleve.NewMatchQuery(browser)
				browserQuery.SetField("browser")
				browserQueries = append(browserQueries, browserQuery)
			}
		}
		if len(browserQueries) == 1 {
			queries = append(queries, browserQueries[0])
		} else if len(browserQueries) > 1 {
			orQuery := bleve.NewDisjunctionQuery(browserQueries...)
			queries = append(queries, orQuery)
		}
	}

	// OS filter
	if req.OS != "" {
		logger.Infof("Adding OS filter: %s", req.OS)
		oses := strings.Split(req.OS, ",")
		var osQueries []query.Query
		for _, os := range oses {
			os = strings.TrimSpace(os)
			if os != "" {
				osQuery := bleve.NewMatchQuery(os)
				osQuery.SetField("os")
				osQueries = append(osQueries, osQuery)
			}
		}
		if len(osQueries) == 1 {
			queries = append(queries, osQueries[0])
		} else if len(osQueries) > 1 {
			orQuery := bleve.NewDisjunctionQuery(osQueries...)
			queries = append(queries, orQuery)
		}
	}

	// Device filter
	if req.Device != "" {
		logger.Infof("Adding device filter: %s", req.Device)
		devices := strings.Split(req.Device, ",")
		var deviceQueries []query.Query
		for _, device := range devices {
			device = strings.TrimSpace(device)
			if device != "" {
				deviceQuery := bleve.NewMatchQuery(device)
				deviceQuery.SetField("device_type")
				deviceQueries = append(deviceQueries, deviceQuery)
			}
		}
		if len(deviceQueries) == 1 {
			queries = append(queries, deviceQueries[0])
		} else if len(deviceQueries) > 1 {
			orQuery := bleve.NewDisjunctionQuery(deviceQueries...)
			queries = append(queries, orQuery)
		}
	}

	// Log path filter (file_path field)
	if req.LogPath != "" {
		logger.Infof("Adding log path filter: %s", req.LogPath)
		filePathQuery := bleve.NewMatchQuery(req.LogPath)
		filePathQuery.SetField("file_path")
		queries = append(queries, filePathQuery)
	}

	// Combine all queries
	if len(queries) == 0 {
		return bleve.NewMatchAllQuery()
	} else if len(queries) == 1 {
		return queries[0]
	} else {
		return bleve.NewConjunctionQuery(queries...)
	}
}

// getStringField safely gets a string field from search results
func (li *LogIndexer) getStringField(fields map[string]interface{}, fieldName string) string {
	if value, ok := fields[fieldName]; ok {
		return cast.ToString(value)
	}
	return ""
}

// getFloatField safely gets a float field from search results
func (li *LogIndexer) getFloatField(fields map[string]interface{}, fieldName string) float64 {
	if value, ok := fields[fieldName]; ok {
		return cast.ToFloat64(value)
	}
	return 0
}

// convertHitToEntry converts a Bleve search hit to an AccessLogEntry
func (li *LogIndexer) convertHitToEntry(hit interface{}) *AccessLogEntry {

	// Try different type assertions for Bleve v2
	switch h := hit.(type) {
	case *search.DocumentMatch:
		entry := &AccessLogEntry{}

		// Extract fields from the hit
		if fields := h.Fields; fields != nil {
			entry.IP = li.getStringField(fields, "ip")
			entry.RegionCode = li.getStringField(fields, "region_code")
			entry.Province = li.getStringField(fields, "province")
			entry.City = li.getStringField(fields, "city")
			entry.Method = li.getStringField(fields, "method")
			entry.Path = li.getStringField(fields, "path")
			entry.Protocol = li.getStringField(fields, "protocol")
			entry.Referer = li.getStringField(fields, "referer")
			entry.UserAgent = li.getStringField(fields, "user_agent")
			entry.Browser = li.getStringField(fields, "browser")
			entry.BrowserVer = li.getStringField(fields, "browser_version")
			entry.OS = li.getStringField(fields, "os")
			entry.OSVersion = li.getStringField(fields, "os_version")
			entry.DeviceType = li.getStringField(fields, "device_type")
			entry.Raw = li.getStringField(fields, "raw")

			// Handle numeric fields
			if statusFloat := li.getFloatField(fields, "status"); statusFloat > 0 {
				entry.Status = int(statusFloat)
			}
			if bytesSent := li.getFloatField(fields, "bytes_sent"); bytesSent > 0 {
				entry.BytesSent = int64(bytesSent)
			}
			entry.RequestTime = li.getFloatField(fields, "request_time")

			// Handle timestamp
			if timestampField := li.getFloatField(fields, "timestamp"); timestampField != 0 {
				entry.Timestamp = int64(timestampField)
			}

		} else {
			logger.Warnf("Hit has no fields: %+v", h)
		}

		return entry

	default:
		logger.Errorf("Unknown hit type: %T, content: %+v", hit, hit)
		return nil
	}
}

// createCacheKey creates a cache key for the given query request
func (li *LogIndexer) createCacheKey(req *QueryRequest) string {
	// Include all search parameters in cache key
	statusStr := ""
	if len(req.Status) > 0 {
		statusStr = fmt.Sprintf("%v", req.Status)
	}

	return fmt.Sprintf("search_%d_%d_%s_%s_%s_%s_%s_%s_%s_%s_%s_%s_%s_%d_%d_%s_%s",
		req.StartTime,
		req.EndTime,
		req.Query,
		req.IP,
		req.Method,
		req.Path,
		req.UserAgent,
		req.Referer,
		req.Browser,
		req.OS,
		req.Device,
		req.LogPath,
		statusStr,
		req.Limit,
		req.Offset,
		req.SortBy,
		req.SortOrder,
	)
}

// mapSortField maps frontend sort field names to Bleve index field names
func (li *LogIndexer) mapSortField(sortBy string) string {
	// Map frontend field names to Bleve index field names
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
		// Default to timestamp if unknown field
		return "timestamp"
	}
}
