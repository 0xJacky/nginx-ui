package nginx_log

import (
	"context"
	"fmt"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

// AnalyticsService provides log analytics functionality
type AnalyticsService struct {
	indexer *LogIndexer
	parser  *LogParser
}

// NewAnalyticsService creates a new analytics service
func NewAnalyticsService() *AnalyticsService {
	// Create user agent parser
	userAgent := NewSimpleUserAgentParser()
	parser := NewLogParser(userAgent)

	return &AnalyticsService{
		parser: parser,
	}
}

// SetIndexer sets the log indexer for the service
func (s *AnalyticsService) SetIndexer(indexer *LogIndexer) {
	s.indexer = indexer
}

// GetIndexer returns the log indexer
func (s *AnalyticsService) GetIndexer() *LogIndexer {
	return s.indexer
}

// AnalyzeLogFile analyzes a log file and returns statistics
func (s *AnalyticsService) AnalyzeLogFile(logPath string) (*LogAnalytics, error) {
	if !IsLogPathUnderWhiteList(logPath) {
		return nil, fmt.Errorf("log path is not under whitelist")
	}

	// Return empty analytics as file parsing is handled by indexer
	return &LogAnalytics{
		TotalRequests: 0,
		StatusCodes:   make(map[int]int),
		Countries:     make(map[string]int),
		Browsers:      make(map[string]int),
		OperatingSystems: make(map[string]int),
		UniqueIPs:     make(map[string]int),
		Devices:       make(map[string]int),
	}, nil
}

// SearchLogs searches for log entries using the indexer
func (s *AnalyticsService) SearchLogs(ctx context.Context, req *QueryRequest) (*QueryResult, error) {
	logger.Infof("AnalyticsService: Searching logs with query: %+v", req)

	if err := s.validateAndNormalizeSearchRequest(req); err != nil {
		return nil, fmt.Errorf("invalid search request: %w", err)
	}

	// Delegate to indexer for actual search
	if s.indexer != nil {
		return s.indexer.SearchLogs(ctx, req)
	}

	return nil, fmt.Errorf("indexer not available")
}

// validateAndNormalizeSearchRequest validates and normalizes the search request
func (s *AnalyticsService) validateAndNormalizeSearchRequest(req *QueryRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	// Validate log path
	if req.LogPath != "" {
		if err := s.ValidateLogPath(req.LogPath); err != nil {
			return fmt.Errorf("invalid log path: %w", err)
		}
	}

	// Ensure positive limit with reasonable limits
	if req.Limit <= 0 {
		req.Limit = 50 // Default limit
	} else if req.Limit > 1000 {
		req.Limit = 1000 // Max limit
	}

	// Ensure non-negative offset
	if req.Offset < 0 {
		req.Offset = 0
	}

	// Validate time range
	if !req.StartTime.IsZero() && !req.EndTime.IsZero() && req.StartTime.After(req.EndTime) {
		return fmt.Errorf("start time cannot be after end time")
	}

	return nil
}

// ValidateLogPath validates that a log path is allowed
func (s *AnalyticsService) ValidateLogPath(logPath string) error {
	if logPath == "" {
		return nil // Empty path is allowed for searching all logs
	}

	// Check whitelist
	if !IsLogPathUnderWhiteList(logPath) {
		return fmt.Errorf("log path %s is not under whitelist", logPath)
	}

	// Additional validation can be added here
	if !isValidLogPath(logPath) {
		return fmt.Errorf("invalid log path format: %s", logPath)
	}

	return nil
}

// GetTimeRange returns the overall time range of all indexed logs
func (s *AnalyticsService) GetTimeRange() (start, end time.Time) {
	if s.indexer != nil {
		return s.indexer.GetTimeRange()
	}
	return time.Time{}, time.Time{}
}

// GetTimeRangeForPath returns the time range for a specific log path
func (s *AnalyticsService) GetTimeRangeForPath(logPath string) (start, end time.Time) {
	if s.indexer != nil {
		return s.indexer.GetTimeRangeForPath(logPath)
	}
	return time.Time{}, time.Time{}
}

// GetTimeRangeFromSummaryStatsForPath returns the time range from summary stats for a specific log path
func (s *AnalyticsService) GetTimeRangeFromSummaryStatsForPath(logPath string) (start, end time.Time) {
	if s.indexer != nil {
		return s.indexer.GetTimeRangeFromSummaryStatsForPath(logPath)
	}
	return time.Time{}, time.Time{}
}

// Global analytics service instance
var analyticsService *AnalyticsService

// InitAnalyticsService initializes the global analytics service
func InitAnalyticsService() {
	analyticsService = NewAnalyticsService()
	logger.Info("Analytics service initialized")
}

// GetAnalyticsService returns the global analytics service instance
func GetAnalyticsService() *AnalyticsService {
	return analyticsService
}

// SetAnalyticsServiceIndexer sets the indexer for the global analytics service
func SetAnalyticsServiceIndexer(indexer *LogIndexer) {
	if analyticsService != nil {
		analyticsService.SetIndexer(indexer)
		logger.Info("Analytics service indexer set")
	}
}