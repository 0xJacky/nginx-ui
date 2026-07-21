package analytics

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/uozi-tech/cosy/logger"
)

// GetDashboardAnalytics generates comprehensive dashboard analytics
func (s *service) GetDashboardAnalytics(ctx context.Context, req *DashboardQueryRequest) (*DashboardAnalytics, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if err := s.ValidateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, fmt.Errorf("invalid time range: %w", err)
	}

	searchReq := &searcher.SearchRequest{
		StartTime:      &req.StartTime,
		EndTime:        &req.EndTime,
		LogPaths:       req.LogPaths,
		UseMainLogPath: true, // Use main_log_path field for efficient log group queries
		IncludeFacets:  true,
		FacetFields:    []string{"browser", "os", "device_type"}, // Removed 'ip' to reduce facet computation
		FacetSize:      50,                                       // Significantly reduced for faster facet computation
		UseCache:       true,
		SortBy:         "timestamp",
		SortOrder:      "desc",
		Limit:          -1, // Facet/aggregation-only query, no documents needed
	}

	// Execute search
	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search logs for dashboard: %w", err)
	}

	// Initialize analytics with empty slices
	analytics := &DashboardAnalytics{}

	// Calculate analytics if we have results
	if result.TotalHits > 0 {
		// For now, use batch queries to get complete data
		analytics.HourlyStats, analytics.DailyStats = s.calculateTimeBucketStats(ctx, req)

		// Use cardinality counter for efficient unique URLs counting
		analytics.TopURLs = s.calculateTopURLsWithCardinality(ctx, req)

		analytics.Browsers = s.calculateBrowserStats(result)
		analytics.OperatingSystems = s.calculateOSStats(result)
		analytics.Devices = s.calculateDeviceStats(result)
	} else {
		// Ensure slices are initialized even if there are no hits
		analytics.HourlyStats = make([]HourlyAccessStats, 0)
		analytics.DailyStats = make([]DailyAccessStats, 0)
		analytics.TopURLs = make([]URLAccessStats, 0)
		analytics.Browsers = make([]BrowserAccessStats, 0)
		analytics.OperatingSystems = make([]OSAccessStats, 0)
		analytics.Devices = make([]DeviceAccessStats, 0)
	}

	// Calculate summary with cardinality counting for accurate unique pages
	analytics.Summary = s.calculateDashboardSummaryWithCardinality(ctx, analytics, result, req)

	return analytics, nil
}

// calculateHourlyStats calculates hourly access statistics.
// Returns 48 hours of data centered around the end_date to support all timezones.
func (s *service) calculateHourlyStats(result *searcher.SearchResult, startTime, endTime int64) []HourlyAccessStats {
	// Use a map with timestamp as key for easier processing
	hourlyMap := make(map[int64]*HourlyAccessStats)
	uniqueIPsPerHour := make(map[int64]map[string]bool)

	// Calculate 48 hours range: from UTC end_date minus 12 hours to plus 36 hours
	// This covers UTC-12 to UTC+14 timezones
	endDate := time.Unix(endTime, 0).UTC()
	endDateStart := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)

	// Create hourly buckets for 48 hours (12 hours before to 36 hours after the UTC date boundary)
	rangeStart := endDateStart.Add(-12 * time.Hour)
	rangeEnd := endDateStart.Add(36 * time.Hour)

	// Initialize hourly buckets
	for t := rangeStart; t.Before(rangeEnd); t = t.Add(time.Hour) {
		timestamp := t.Unix()
		hourlyMap[timestamp] = &HourlyAccessStats{
			Hour:      t.Hour(),
			UV:        0,
			PV:        0,
			Timestamp: timestamp,
		}
		uniqueIPsPerHour[timestamp] = make(map[string]bool)
	}

	// Process search results - count hits within the 48-hour window
	for _, hit := range result.Hits {
		if timestampField, ok := hit.Fields["timestamp"]; ok {
			if timestampFloat, ok := timestampField.(float64); ok {
				timestamp := int64(timestampFloat)

				// Check if this hit falls within our 48-hour window
				if timestamp >= rangeStart.Unix() && timestamp < rangeEnd.Unix() {
					// Round down to the hour
					t := time.Unix(timestamp, 0).UTC()
					hourTimestamp := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC).Unix()

					if stats, exists := hourlyMap[hourTimestamp]; exists {
						stats.PV++
						if ipField, ok := hit.Fields["ip"]; ok {
							if ip, ok := ipField.(string); ok && ip != "" {
								if !uniqueIPsPerHour[hourTimestamp][ip] {
									uniqueIPsPerHour[hourTimestamp][ip] = true
									stats.UV++
								}
							}
						}
					}
				}
			}
		}
	}

	// Convert to slice and sort by timestamp
	var stats []HourlyAccessStats
	for _, stat := range hourlyMap {
		stats = append(stats, *stat)
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Timestamp < stats[j].Timestamp
	})

	return stats
}

// calculateDailyStats calculates daily access statistics
func (s *service) calculateDailyStats(result *searcher.SearchResult, startTime, endTime int64) []DailyAccessStats {
	dailyMap := make(map[string]*DailyAccessStats)
	uniqueIPsPerDay := make(map[string]map[string]bool)

	// Initialize daily buckets for the entire time range
	start := time.Unix(startTime, 0)
	end := time.Unix(endTime, 0)
	for t := start; t.Before(end) || t.Equal(end); t = t.AddDate(0, 0, 1) {
		dateStr := t.Format("2006-01-02")
		if _, exists := dailyMap[dateStr]; !exists {
			dailyMap[dateStr] = &DailyAccessStats{
				Date:      dateStr,
				UV:        0,
				PV:        0,
				Timestamp: t.Unix(),
			}
			uniqueIPsPerDay[dateStr] = make(map[string]bool)
		}
	}

	// Process search results
	for _, hit := range result.Hits {
		if timestampField, ok := hit.Fields["timestamp"]; ok {
			if timestampFloat, ok := timestampField.(float64); ok {
				timestamp := int64(timestampFloat)
				t := time.Unix(timestamp, 0)
				dateStr := t.Format("2006-01-02")

				if stats, exists := dailyMap[dateStr]; exists {
					stats.PV++
					if ipField, ok := hit.Fields["ip"]; ok {
						if ip, ok := ipField.(string); ok && ip != "" {
							if !uniqueIPsPerDay[dateStr][ip] {
								uniqueIPsPerDay[dateStr][ip] = true
								stats.UV++
							}
						}
					}
				}
			}
		}
	}

	// Convert to slice and sort
	var stats []DailyAccessStats
	for _, stat := range dailyMap {
		stats = append(stats, *stat)
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Timestamp < stats[j].Timestamp
	})

	return stats
}

// calculateTopURLs calculates top URL statistics from facets (legacy method)
func (s *service) calculateTopURLs(result *searcher.SearchResult) []URLAccessStats {
	if facet, ok := result.Facets["path_exact"]; ok {
		logger.Infof("📊 Facet-based URL calculation: facet.Total=%d, TotalHits=%d",
			facet.Total, result.TotalHits)

		urlStats := calculateTopFieldStats(facet, int(result.TotalHits), func(term string, count int, percent float64) URLAccessStats {
			return URLAccessStats{URL: term, Visits: count, Percent: percent}
		})

		logger.Infof("📈 Calculated %d URL stats from facet", len(urlStats))
		return urlStats
	} else {
		logger.Errorf("❌ path_exact facet not found in search results")
		return []URLAccessStats{}
	}
}

// calculateTopURLsWithCardinality calculates top URL statistics using facet-based approach
// Always returns actual top URLs with their visit counts instead of just a summary
func (s *service) calculateTopURLsWithCardinality(ctx context.Context, req *DashboardQueryRequest) []URLAccessStats {
	// Always use facet-based calculation to get actual top URLs with visit counts
	searchReq := &searcher.SearchRequest{
		StartTime:      &req.StartTime,
		EndTime:        &req.EndTime,
		LogPaths:       req.LogPaths,
		UseMainLogPath: true, // Use main_log_path for efficient log group queries
		IncludeFacets:  true,
		FacetFields:    []string{"path_exact"},
		FacetSize:      100, // Reasonable facet size to get top URLs
		UseCache:       true,
		Limit:          -1, // Facet-only query, no documents needed
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		logger.Errorf("Failed to search for URL facets: %v", err)
		return []URLAccessStats{}
	}

	// Get actual top URLs with visit counts
	return s.calculateTopURLs(result)
}

// calculateBrowserStats calculates browser statistics from facets
func (s *service) calculateBrowserStats(result *searcher.SearchResult) []BrowserAccessStats {
	return calculateTopFieldStats(result.Facets["browser"], int(result.TotalHits), func(term string, count int, percent float64) BrowserAccessStats {
		return BrowserAccessStats{Browser: term, Count: count, Percent: percent}
	})
}

// calculateOSStats calculates operating system statistics from facets
func (s *service) calculateOSStats(result *searcher.SearchResult) []OSAccessStats {
	return calculateTopFieldStats(result.Facets["os"], int(result.TotalHits), func(term string, count int, percent float64) OSAccessStats {
		return OSAccessStats{OS: term, Count: count, Percent: percent}
	})
}

// calculateDeviceStats calculates device statistics from facets
func (s *service) calculateDeviceStats(result *searcher.SearchResult) []DeviceAccessStats {
	return calculateTopFieldStats(result.Facets["device_type"], int(result.TotalHits), func(term string, count int, percent float64) DeviceAccessStats {
		return DeviceAccessStats{Device: term, Count: count, Percent: percent}
	})
}

// calculateTopFieldStats is a generic function to calculate top N items from a facet result.
func calculateTopFieldStats[T any](
	facet *searcher.Facet,
	totalHits int,
	creator func(term string, count int, percent float64) T,
) []T {
	if facet == nil || totalHits == 0 {
		return []T{}
	}

	var items []T
	for _, term := range facet.Terms {
		percent := float64(term.Count) / float64(totalHits) * 100
		items = append(items, creator(term.Term, term.Count, percent))
	}
	return items
}

// calculateDashboardSummary calculates summary statistics
func (s *service) calculateDashboardSummary(analytics *DashboardAnalytics, result *searcher.SearchResult) DashboardSummary {
	// Calculate total UV from IP facet, which is now reliable.
	totalUV := 0
	if result.Facets != nil {
		if ipFacet, ok := result.Facets["ip"]; ok {
			// The total number of unique terms in the facet is the UV count.
			totalUV = ipFacet.Total
		}
	}

	totalPV := int(result.TotalHits)

	// Calculate average daily UV and PV
	var avgDailyUV, avgDailyPV float64
	if len(analytics.DailyStats) > 0 {
		var sumPV int
		for _, daily := range analytics.DailyStats {
			sumPV += daily.PV
		}
		// Use total unique visitors divided by number of days for accurate daily UV average
		// The totalUV represents unique visitors across the entire period, not sum of daily UVs
		avgDailyUV = float64(totalUV) / float64(len(analytics.DailyStats))
		avgDailyPV = float64(sumPV) / float64(len(analytics.DailyStats))
	}

	// Find peak hour
	var peakHour, peakHourTraffic int
	for _, hourly := range analytics.HourlyStats {
		if hourly.PV > peakHourTraffic {
			peakHour = hourly.Hour
			peakHourTraffic = hourly.PV
		}
	}

	return DashboardSummary{
		TotalUV:         totalUV,
		TotalPV:         totalPV,
		AvgDailyUV:      avgDailyUV,
		AvgDailyPV:      avgDailyPV,
		PeakHour:        peakHour,
		PeakHourTraffic: peakHourTraffic,
	}
}

// calculateDashboardSummaryWithCardinality calculates enhanced summary statistics using cardinality counters
func (s *service) calculateDashboardSummaryWithCardinality(ctx context.Context, analytics *DashboardAnalytics, result *searcher.SearchResult, req *DashboardQueryRequest) DashboardSummary {
	// Start with the basic summary but we'll override the UV calculation
	summary := s.calculateDashboardSummary(analytics, result)

	// Use cardinality counter for accurate unique visitor (UV) counting if available
	cardinalityCounter := s.getCardinalityCounter()
	if cardinalityCounter != nil {
		// Count unique IPs (visitors) using cardinality counter instead of limited facet
		uvCardReq := &searcher.CardinalityRequest{
			Field:          "ip",
			StartTime:      &req.StartTime,
			EndTime:        &req.EndTime,
			LogPaths:       req.LogPaths,
			UseMainLogPath: true, // Use main_log_path for efficient log group queries
		}

		if uvResult, err := cardinalityCounter.Count(ctx, uvCardReq); err == nil {
			// Override the facet-limited UV count with accurate cardinality count
			summary.TotalUV = int(uvResult.Cardinality)

			// Recalculate average daily UV with accurate count
			if len(analytics.DailyStats) > 0 {
				summary.AvgDailyUV = float64(summary.TotalUV) / float64(len(analytics.DailyStats))
			}

		} else {
			logger.Errorf("Failed to count unique visitors with cardinality counter: %v", err)
		}
	} else {
		logger.Warnf("Counter not available, UV count limited by facet size to %d", summary.TotalUV)
	}

	return summary
}

// calculateTimeBucketStats computes hourly and daily UV/PV statistics in a
// single pass over the matching documents. Pagination uses a SearchAfter
// cursor on the (timestamp, _id) sort key: each page costs O(page) instead of
// the O(offset+page) of offset pagination, so a full scan stays linear in the
// number of documents.
func (s *service) calculateTimeBucketStats(ctx context.Context, req *DashboardQueryRequest) ([]HourlyAccessStats, []DailyAccessStats) {
	// Daily buckets cover the requested range (dates in server-local time,
	// matching the rest of the dashboard).
	dailyMap := make(map[string]*DailyAccessStats)
	uniqueIPsPerDay := make(map[string]map[string]bool)

	start := time.Unix(req.StartTime, 0)
	end := time.Unix(req.EndTime, 0)
	for t := start; t.Before(end) || t.Equal(end); t = t.AddDate(0, 0, 1) {
		dateStr := t.Format("2006-01-02")
		if _, exists := dailyMap[dateStr]; !exists {
			dailyMap[dateStr] = &DailyAccessStats{
				Date:      dateStr,
				Timestamp: t.Unix(),
			}
			uniqueIPsPerDay[dateStr] = make(map[string]bool)
		}
	}

	// Hourly buckets cover the requested range plus a timezone buffer
	// (12 hours on each side, covering UTC-12 to UTC+12).
	hourlyMap := make(map[int64]*HourlyAccessStats)
	uniqueIPsPerHour := make(map[int64]map[string]bool)

	rangeStart := time.Unix(req.StartTime, 0).UTC().Add(-12 * time.Hour)
	rangeEnd := time.Unix(req.EndTime, 0).UTC().Add(12 * time.Hour)
	for t := rangeStart; t.Before(rangeEnd); t = t.Add(time.Hour) {
		timestamp := t.Unix()
		hourlyMap[timestamp] = &HourlyAccessStats{
			Hour:      t.Hour(),
			Timestamp: timestamp,
		}
		uniqueIPsPerHour[timestamp] = make(map[string]bool)
	}

	// One scan over the wider (hourly) range feeds both bucket sets; documents
	// outside the daily range simply miss the daily map and are skipped there.
	scanStart := rangeStart.Unix()
	scanEnd := rangeEnd.Unix()
	const batchSize = 10000

	var searchAfter []string
	totalProcessed := 0

	for {
		searchReq := &searcher.SearchRequest{
			StartTime:      &scanStart,
			EndTime:        &scanEnd,
			LogPaths:       req.LogPaths,
			UseMainLogPath: true, // Use main_log_path for efficient log group queries
			Limit:          batchSize,
			SearchAfter:    searchAfter,
			SortBy:         "timestamp",
			SortOrder:      "asc",
			Fields:         []string{"timestamp", "ip"},
			UseCache:       false, // Don't cache intermediate scan pages
		}

		result, err := s.searcher.Search(ctx, searchReq)
		if err != nil {
			logger.Errorf("Failed to fetch time-bucket batch (processed %d): %v", totalProcessed, err)
			break
		}

		for _, hit := range result.Hits {
			timestampField, ok := hit.Fields["timestamp"]
			if !ok {
				continue
			}
			timestampFloat, ok := timestampField.(float64)
			if !ok {
				continue
			}
			timestamp := int64(timestampFloat)

			var ip string
			if ipField, ok := hit.Fields["ip"]; ok {
				ip, _ = ipField.(string)
			}

			// Daily bucket (server-local date)
			dateStr := time.Unix(timestamp, 0).Format("2006-01-02")
			if stats, exists := dailyMap[dateStr]; exists {
				stats.PV++
				if ip != "" && !uniqueIPsPerDay[dateStr][ip] {
					uniqueIPsPerDay[dateStr][ip] = true
					stats.UV++
				}
			}

			// Hourly bucket (UTC hour)
			t := time.Unix(timestamp, 0).UTC()
			hourTimestamp := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC).Unix()
			if stats, exists := hourlyMap[hourTimestamp]; exists {
				stats.PV++
				if ip != "" && !uniqueIPsPerHour[hourTimestamp][ip] {
					uniqueIPsPerHour[hourTimestamp][ip] = true
					stats.UV++
				}
			}
		}

		totalProcessed += len(result.Hits)

		if len(result.Hits) < batchSize {
			break
		}

		lastHit := result.Hits[len(result.Hits)-1]
		if len(lastHit.Sort) == 0 {
			logger.Warnf("Time-bucket scan: last hit carries no sort values, cannot continue pagination (processed %d)", totalProcessed)
			break
		}
		searchAfter = lastHit.Sort
	}

	logger.Debugf("Time-bucket stats completed: %d records into %d hourly / %d daily buckets",
		totalProcessed, len(hourlyMap), len(dailyMap))

	// Convert to sorted slices
	hourlyStats := make([]HourlyAccessStats, 0, len(hourlyMap))
	for _, stat := range hourlyMap {
		hourlyStats = append(hourlyStats, *stat)
	}
	sort.Slice(hourlyStats, func(i, j int) bool {
		return hourlyStats[i].Timestamp < hourlyStats[j].Timestamp
	})

	dailyStats := make([]DailyAccessStats, 0, len(dailyMap))
	for _, stat := range dailyMap {
		dailyStats = append(dailyStats, *stat)
	}
	sort.Slice(dailyStats, func(i, j int) bool {
		return dailyStats[i].Timestamp < dailyStats[j].Timestamp
	})

	return hourlyStats, dailyStats
}
