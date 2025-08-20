package nginx_log

import (
	"context"
	"fmt"

	"github.com/uozi-tech/cosy/logger"
)

// GetDashboardAnalytics generates comprehensive dashboard analytics
func (s *AnalyticsService) GetDashboardAnalytics(ctx context.Context, req *DashboardQueryRequest) (*DashboardAnalytics, error) {
	if s.indexer == nil {
		return nil, fmt.Errorf("log indexer not available")
	}

	// Build comprehensive search request to get all entries in the time range
	searchReq := &QueryRequest{
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		LogPath:   req.LogPath,
		Limit:     0, // No limit - get all entries
		Offset:    0,
		SortBy:    "timestamp",
		SortOrder: "asc",
	}

	// Execute search to get all log entries
	result, err := s.indexer.SearchLogs(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search logs: %w", err)
	}

	// Calculate dashboard analytics - ensure we initialize with empty arrays using make
	analytics := &DashboardAnalytics{
		HourlyStats:      make([]HourlyAccessStats, 0),
		DailyStats:       make([]DailyAccessStats, 0),
		TopURLs:          make([]URLAccessStats, 0),
		Browsers:         make([]BrowserAccessStats, 0),
		OperatingSystems: make([]OSAccessStats, 0),
		Devices:          make([]DeviceAccessStats, 0),
	}

	// Only calculate if we have entries
	if len(result.Entries) > 0 {
		analytics.HourlyStats = s.calculateHourlyStats(result.Entries, req.StartTime, req.EndTime)
		analytics.DailyStats = s.calculateDailyStats(result.Entries, req.StartTime, req.EndTime)
		analytics.TopURLs = s.calculateTopURLs(result.Entries)
		analytics.Browsers = s.calculateBrowserStats(result.Entries)
		analytics.OperatingSystems = s.calculateOSStats(result.Entries)
		analytics.Devices = s.calculateDeviceStats(result.Entries)
	}

	// Calculate summary statistics
	analytics.Summary = s.calculateDashboardSummary(analytics, result.Entries)

	return analytics, nil
}

// GetDashboardAnalyticsFromStats retrieves dashboard analytics using Bleve aggregations
func (s *AnalyticsService) GetDashboardAnalyticsFromStats(ctx context.Context, req *DashboardQueryRequest) (*DashboardAnalytics, error) {
	// Use Bleve stats service instead of database statistics
	bleveStatsService := GetBleveStatsService()
	if bleveStatsService == nil {
		return nil, fmt.Errorf("Bleve stats service not available")
	}

	logger.Infof("Using Bleve-based statistics for log path: %s", req.LogPath)

	// Get analytics from Bleve index
	analytics, err := bleveStatsService.GetDashboardAnalytics(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get dashboard analytics from Bleve: %w", err)
	}

	logger.Debugf("Successfully retrieved dashboard analytics from Bleve")
	return analytics, nil
}

// calculateDashboardSummary calculates summary statistics for the dashboard
func (s *AnalyticsService) calculateDashboardSummary(analytics *DashboardAnalytics, entries []*AccessLogEntry) DashboardSummary {
	// Calculate total UV and PV
	uniqueIPs := make(map[string]bool)
	for _, entry := range entries {
		uniqueIPs[entry.IP] = true
	}

	totalUV := len(uniqueIPs)
	totalPV := len(entries)

	// Calculate average daily UV and PV
	var avgDailyUV, avgDailyPV float64
	if len(analytics.DailyStats) > 0 {
		totalDays := len(analytics.DailyStats)
		var sumUV, sumPV int
		for _, daily := range analytics.DailyStats {
			sumUV += daily.UV
			sumPV += daily.PV
		}
		avgDailyUV = float64(sumUV) / float64(totalDays)
		avgDailyPV = float64(sumPV) / float64(totalDays)
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