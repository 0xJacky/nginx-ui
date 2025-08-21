package nginx_log

import (
	"context"
	"fmt"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/uozi-tech/cosy/logger"
)

// BleveStatsService provides log statistics using Bleve aggregations only
type BleveStatsService struct {
	indexer *LogIndexer
}

// NewBleveStatsService creates a new Bleve-based statistics service
func NewBleveStatsService() *BleveStatsService {
	return &BleveStatsService{}
}

// SetIndexer sets the log indexer for the service
func (s *BleveStatsService) SetIndexer(indexer *LogIndexer) {
	s.indexer = indexer
}

// GetDashboardAnalytics generates comprehensive dashboard analytics using Bleve aggregations
func (s *BleveStatsService) GetDashboardAnalytics(ctx context.Context, req *DashboardQueryRequest) (*DashboardAnalytics, error) {
	if s.indexer == nil {
		logger.Error("BleveStatsService: log indexer not available")
		return nil, fmt.Errorf("log indexer not available")
	}

	if s.indexer.index == nil {
		logger.Error("BleveStatsService: Bleve index is nil")
		return nil, fmt.Errorf("Bleve index not available")
	}

	logger.Infof("BleveStatsService: Starting dashboard analytics for log_path='%s', start=%v, end=%v",
		req.LogPath, req.StartTime, req.EndTime)


	// Build time range query
	timeQuery := s.buildTimeRangeQuery(req.StartTime, req.EndTime)

	// Add log path filter if specified
	var searchQuery query.Query = timeQuery
	if req.LogPath != "" {
		// Use proper field-specific MatchQuery with keyword analyzer (Bleve-layer filtering)
		boolQuery := bleve.NewBooleanQuery()

		// Add time range query (if it's not just MatchAllQuery)
		if timeQuery != nil {
			boolQuery.AddMust(timeQuery)
		}

		// Use MatchQuery with field specification for exact file_path matching
		filePathMatchQuery := bleve.NewMatchQuery(req.LogPath)
		filePathMatchQuery.SetField("file_path") // Now this should work with TextFieldMapping + keyword analyzer
		boolQuery.AddMust(filePathMatchQuery)

		searchQuery = boolQuery
	}

	// Initialize result with empty arrays to ensure JSON structure
	analytics := &DashboardAnalytics{
		HourlyStats:      make([]HourlyAccessStats, 0),
		DailyStats:       make([]DailyAccessStats, 0),
		TopURLs:          make([]URLAccessStats, 0),
		Browsers:         make([]BrowserAccessStats, 0),
		OperatingSystems: make([]OSAccessStats, 0),
		Devices:          make([]DeviceAccessStats, 0),
	}

	// Execute various aggregation queries in parallel
	hourlyStats, err := s.calculateHourlyStatsFromBleve(ctx, searchQuery, req.StartTime, req.EndTime)
	if err != nil {
		logger.Warnf("Failed to calculate hourly stats: %v", err)
	} else {
		analytics.HourlyStats = hourlyStats
	}

	dailyStats, err := s.calculateDailyStatsFromBleve(ctx, searchQuery, req.StartTime, req.EndTime)
	if err != nil {
		logger.Warnf("Failed to calculate daily stats: %v", err)
	} else {
		analytics.DailyStats = dailyStats
	}

	topURLs, err := s.calculateTopURLsFromBleve(ctx, searchQuery)
	if err != nil {
		logger.Warnf("Failed to calculate top URLs: %v", err)
	} else {
		analytics.TopURLs = topURLs
	}

	browsers, err := s.calculateBrowserStatsFromBleve(ctx, searchQuery)
	if err != nil {
		logger.Warnf("Failed to calculate browser stats: %v", err)
	} else {
		analytics.Browsers = browsers
	}

	osStats, err := s.calculateOSStatsFromBleve(ctx, searchQuery)
	if err != nil {
		logger.Warnf("Failed to calculate OS stats: %v", err)
	} else {
		analytics.OperatingSystems = osStats
	}

	deviceStats, err := s.calculateDeviceStatsFromBleve(ctx, searchQuery)
	if err != nil {
		logger.Warnf("Failed to calculate device stats: %v", err)
	} else {
		analytics.Devices = deviceStats
	}

	// Calculate summary statistics using the same algorithm as search interface
	summaryStats, err := s.indexer.calculateSummaryStatsFromQuery(ctx, searchQuery)
	if err != nil {
		logger.Warnf("Failed to calculate summary stats: %v", err)
		// Create empty summary on error
		analytics.Summary = DashboardSummary{}
	} else {
		// Convert SummaryStats to DashboardSummary format
		analytics.Summary = DashboardSummary{
			TotalUV:         summaryStats.UV,
			TotalPV:         summaryStats.PV,
			AvgDailyUV:      s.calculateAvgDailyUVFromStats(analytics),
			AvgDailyPV:      s.calculateAvgDailyPVFromStats(analytics),
			PeakHour:        s.findPeakHourFromStats(analytics),
			PeakHourTraffic: s.findPeakHourTrafficFromStats(analytics),
		}
	}

	return analytics, nil
}

// buildTimeRangeQuery builds a time range query for Bleve
func (s *BleveStatsService) buildTimeRangeQuery(startTime, endTime int64) query.Query {
	// If both times are zero or the range is too wide, return match all query
	if startTime == 0 && endTime == 0 {
		return bleve.NewMatchAllQuery()
	}

	// Check if the time range is reasonable (same as search interface)
	if startTime != 0 && endTime != 0 {
		if endTime-startTime >= 400*24*3600 { // More than ~400 days in seconds
			return bleve.NewMatchAllQuery()
		}
	}

	// Build proper time range query
	var timeQuery query.Query
	if startTime != 0 && endTime != 0 {
		// Add 1 second to endTime to ensure boundary values are included
		inclusiveEndTime := endTime + 1
		startFloat := float64(startTime)
		endFloat := float64(inclusiveEndTime)
		timeQuery = bleve.NewNumericRangeQuery(&startFloat, &endFloat)
		timeQuery.(*query.NumericRangeQuery).SetField("timestamp")
	} else if startTime != 0 {
		startFloat := float64(startTime)
		timeQuery = bleve.NewNumericRangeQuery(&startFloat, nil)
		timeQuery.(*query.NumericRangeQuery).SetField("timestamp")
	} else if endTime != 0 {
		// Add 1 second to endTime to ensure boundary values are included
		inclusiveEndTime := endTime + 1
		endFloat := float64(inclusiveEndTime)
		timeQuery = bleve.NewNumericRangeQuery(nil, &endFloat)
		timeQuery.(*query.NumericRangeQuery).SetField("timestamp")
	} else {
		return bleve.NewMatchAllQuery()
	}

	return timeQuery
}

