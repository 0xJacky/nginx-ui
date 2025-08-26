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
		StartTime:     &req.StartTime,
		EndTime:       &req.EndTime,
		LogPaths:      req.LogPaths,
		IncludeFacets: true,
		FacetFields:   []string{"path_exact", "browser", "os", "device_type", "ip"},
		FacetSize:     10,
		UseCache:      true,
		SortBy:        "timestamp",
		SortOrder:     "desc",
	}

	// Execute search
	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search logs for dashboard: %w", err)
	}

	// --- DIAGNOSTIC LOGGING ---
	logger.Debugf("Dashboard search completed. Total Hits: %d, Returned Hits: %d, Facets: %d", 
		result.TotalHits, len(result.Hits), len(result.Facets))
	if result.TotalHits > uint64(len(result.Hits)) {
		logger.Warnf("Dashboard sampling: using %d/%d documents for time calculations (%.1f%% coverage)", 
			len(result.Hits), result.TotalHits, float64(len(result.Hits))/float64(result.TotalHits)*100)
	}
	// --- END DIAGNOSTIC LOGGING ---

	// Initialize analytics with empty slices
	analytics := &DashboardAnalytics{}

	// Calculate analytics if we have results
	if result.TotalHits > 0 {
		analytics.HourlyStats = s.calculateHourlyStats(result, req.StartTime, req.EndTime)
		analytics.DailyStats = s.calculateDailyStats(result, req.StartTime, req.EndTime)
		analytics.TopURLs = s.calculateTopURLs(result)
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

	// Calculate summary
	analytics.Summary = s.calculateDashboardSummary(analytics, result)

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

// calculateTopURLs calculates top URL statistics from facets
func (s *service) calculateTopURLs(result *searcher.SearchResult) []URLAccessStats {
	return calculateTopFieldStats(result.Facets["path_exact"], int(result.TotalHits), func(term string, count int, percent float64) URLAccessStats {
		return URLAccessStats{URL: term, Visits: count, Percent: percent}
	})
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
		var sumUV, sumPV int
		for _, daily := range analytics.DailyStats {
			sumUV += daily.UV
			sumPV += daily.PV
		}
		if len(analytics.DailyStats) > 0 {
			avgDailyUV = float64(sumUV) / float64(len(analytics.DailyStats))
			avgDailyPV = float64(sumPV) / float64(len(analytics.DailyStats))
		}
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
