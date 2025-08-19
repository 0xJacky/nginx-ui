package nginx_log

import (
	"context"
	"fmt"
	"time"

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

	// First, check total document count in index
	docCount, err := s.indexer.index.DocCount()
	if err != nil {
		logger.Errorf("BleveStatsService: Failed to get document count: %v", err)
	} else {
		logger.Infof("BleveStatsService: Total documents in index: %d", docCount)
	}

	// Debug: Let's see what file_path values are actually in the index
	debugSearchReq := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
	debugSearchReq.Size = 5
	debugSearchReq.Fields = []string{"file_path"}
	debugResult, err := s.indexer.index.Search(debugSearchReq)
	if err != nil {
		logger.Errorf("BleveStatsService: Failed to debug search: %v", err)
	} else {
		logger.Infof("BleveStatsService: Debug - sample file_path values in index:")
		for i, hit := range debugResult.Hits {
			if filePathField, ok := hit.Fields["file_path"]; ok {
				logger.Infof("  [%d] file_path: '%v'", i, filePathField)
			} else {
				logger.Infof("  [%d] file_path: <missing>", i)
			}
		}
	}

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
		logger.Infof("BleveStatsService: Using BooleanQuery with field-specific file_path MatchQuery for '%s'", req.LogPath)
	} else {
		logger.Info("BleveStatsService: No log path filter, using time query only")
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
func (s *BleveStatsService) buildTimeRangeQuery(startTime, endTime time.Time) query.Query {
	// If both times are zero or the range is too wide, return match all query
	if startTime.IsZero() && endTime.IsZero() {
		logger.Info("BleveStatsService: No time range specified, using MatchAll query")
		return bleve.NewMatchAllQuery()
	}

	// Check if the time range is reasonable (same as search interface)
	if !startTime.IsZero() && !endTime.IsZero() {
		if endTime.Sub(startTime) >= 400*24*time.Hour { // More than ~400 days
			logger.Infof("BleveStatsService: Time range too wide (%v), using MatchAll query", endTime.Sub(startTime))
			return bleve.NewMatchAllQuery()
		}
	}

	// Build proper time range query
	var timeQuery query.Query
	if !startTime.IsZero() && !endTime.IsZero() {
		// Add 1 millisecond to endTime to ensure boundary values are included
		// This fixes the issue where records with exact endTime are excluded due to exclusive upper bound
		inclusiveEndTime := endTime.Add(1 * time.Millisecond)
		logger.Infof("BleveStatsService: Using time range filter: %s to %s (inclusive)", startTime.Format(time.RFC3339), inclusiveEndTime.Format(time.RFC3339))
		timeQuery = bleve.NewDateRangeQuery(startTime, inclusiveEndTime)
		timeQuery.(*query.DateRangeQuery).SetField("timestamp")
	} else if !startTime.IsZero() {
		logger.Infof("BleveStatsService: Using start time filter: %s", startTime.Format(time.RFC3339))
		timeQuery = bleve.NewDateRangeQuery(startTime, time.Time{})
		timeQuery.(*query.DateRangeQuery).SetField("timestamp")
	} else if !endTime.IsZero() {
		// Add 1 millisecond to endTime to ensure boundary values are included
		inclusiveEndTime := endTime.Add(1 * time.Millisecond)
		logger.Infof("BleveStatsService: Using end time filter: %s (inclusive)", inclusiveEndTime.Format(time.RFC3339))
		timeQuery = bleve.NewDateRangeQuery(time.Time{}, inclusiveEndTime)
		timeQuery.(*query.DateRangeQuery).SetField("timestamp")
	} else {
		logger.Info("BleveStatsService: No valid time range, using MatchAll query")
		return bleve.NewMatchAllQuery()
	}

	return timeQuery
}