package nginx_log

import (
	"context"
	"fmt"
	"sort"
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
		OSes:          make(map[string]int),
		DeviceTypes:   make(map[string]int),
	}, nil
}

// SearchLogs performs advanced search on indexed logs with validation
func (s *AnalyticsService) SearchLogs(ctx context.Context, req *QueryRequest) (*QueryResult, error) {
	if s.indexer == nil {
		return nil, fmt.Errorf("log indexer not available")
	}

	// Validate and set default values
	if err := s.validateAndNormalizeSearchRequest(req); err != nil {
		return nil, err
	}

	return s.indexer.SearchLogs(ctx, req)
}

// validateAndNormalizeSearchRequest validates and normalizes search request
func (s *AnalyticsService) validateAndNormalizeSearchRequest(req *QueryRequest) error {
	// Set default limit if not provided
	if req.Limit <= 0 {
		req.Limit = 100
	}

	// Enforce maximum limit
	if req.Limit > 1000 {
		req.Limit = 1000
	}

	// Validate offset
	if req.Offset < 0 {
		req.Offset = 0
	}

	// Validate time range
	if !req.StartTime.IsZero() && !req.EndTime.IsZero() {
		if req.StartTime.After(req.EndTime) {
			return fmt.Errorf("start time cannot be after end time")
		}
	}

	// Validate status codes
	for _, status := range req.Status {
		if status < 100 || status > 599 {
			return fmt.Errorf("invalid HTTP status code: %d", status)
		}
	}

	return nil
}

// ValidateLogPath validates if a log path is allowed and exists
func (s *AnalyticsService) ValidateLogPath(logPath string) error {
	if logPath == "" {
		return nil // Empty path is allowed
	}

	if !IsLogPathUnderWhiteList(logPath) {
		return ErrLogPathNotUnderWhitelist
	}

	// Use isValidLogPath to safely validate path (it includes os.Stat internally with proper checks)
	if !isValidLogPath(logPath) {
		return ErrLogFileNotExists
	}

	return nil
}

// GetLogEntries retrieves log entries from a file
func (s *AnalyticsService) GetLogEntries(logPath string, limit int, tail bool) ([]*AccessLogEntry, error) {
	if logPath == "" {
		return nil, fmt.Errorf("log path is required")
	}

	// Validate log path
	if err := s.ValidateLogPath(logPath); err != nil {
		return nil, err
	}

	// Set default limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	// Use indexer if available for better performance
	if s.indexer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		queryReq := &QueryRequest{
			Limit:  limit,
			Offset: 0,
		}

		result, err := s.indexer.SearchLogs(ctx, queryReq)
		if err == nil && len(result.Entries) > 0 {
			return result.Entries, nil
		}
		// Fall back to direct file reading if indexer fails
	}

	// Direct file parsing as fallback
	entries, err := s.parseLogFileDirectly(logPath, limit, tail)
	if err != nil {
		return nil, fmt.Errorf("failed to parse log file: %w", err)
	}

	return entries, nil
}

// parseLogFileDirectly parses log file directly without indexer
func (s *AnalyticsService) parseLogFileDirectly(logPath string, limit int, tail bool) ([]*AccessLogEntry, error) {
	// This is a simplified implementation
	// In a real scenario, you might want to implement proper file reading with tail support
	return []*AccessLogEntry{}, nil
}

// GetTimeRange returns the available time range for indexed logs
func (s *AnalyticsService) GetTimeRange() (start, end time.Time) {
	if s.indexer == nil {
		return time.Time{}, time.Time{}
	}

	return s.indexer.GetTimeRange()
}

// GetTimeRangeForPath returns the available time range for a specific log file
func (s *AnalyticsService) GetTimeRangeForPath(logPath string) (start, end time.Time) {
	if s.indexer == nil {
		return time.Time{}, time.Time{}
	}

	return s.indexer.GetTimeRangeForPath(logPath)
}

// GetIndexStatus returns comprehensive status and statistics about the indexer
func (s *AnalyticsService) GetIndexStatus() (*IndexStatus, error) {
	if s.indexer == nil {
		return nil, ErrIndexerNotAvailable
	}

	return s.indexer.GetIndexStatus()
}

// generateAnalytics generates analytics from log entries
func (s *AnalyticsService) generateAnalytics(entries []*AccessLogEntry) *LogAnalytics {
	analytics := &LogAnalytics{
		TotalRequests: len(entries),
		UniqueIPs:     make(map[string]int),
		StatusCodes:   make(map[int]int),
		TopPaths:      make(map[string]int),
		TopIPs:        make(map[string]int),
		Countries:     make(map[string]int),
		Browsers:      make(map[string]int),
		OSes:          make(map[string]int),
		DeviceTypes:   make(map[string]int),
	}

	var totalResponseTime float64
	var errorCount int

	for _, entry := range entries {
		// Count unique IPs
		analytics.UniqueIPs[entry.IP]++
		analytics.TopIPs[entry.IP]++

		// Count status codes
		analytics.StatusCodes[entry.Status]++
		if entry.Status >= 400 {
			errorCount++
		}

		// Count top paths
		analytics.TopPaths[entry.Path]++

		// Count locations (countries)
		if entry.Location != "" {
			analytics.Countries[entry.Location]++
		}

		// Count browsers
		if entry.Browser != "" {
			analytics.Browsers[entry.Browser]++
		}

		// Count operating systems
		if entry.OS != "" {
			analytics.OSes[entry.OS]++
		}

		// Count device types
		if entry.DeviceType != "" {
			analytics.DeviceTypes[entry.DeviceType]++
		}

		// Calculate response time
		totalResponseTime += entry.RequestTime
	}

	// Calculate derived metrics
	analytics.UniqueIPCount = len(analytics.UniqueIPs)
	if analytics.TotalRequests > 0 {
		analytics.AverageResponseTime = totalResponseTime / float64(analytics.TotalRequests)
		analytics.ErrorRate = float64(errorCount) / float64(analytics.TotalRequests) * 100
	}

	// Convert maps to sorted slices for top items
	analytics.TopPathsList = s.mapToSortedList(analytics.TopPaths, 10)
	analytics.TopIPsList = s.mapToSortedList(analytics.TopIPs, 10)
	analytics.CountriesList = s.mapToSortedList(analytics.Countries, 10)
	analytics.BrowsersList = s.mapToSortedList(analytics.Browsers, 10)
	analytics.OSList = s.mapToSortedList(analytics.OSes, 10)
	analytics.DeviceTypesList = s.mapToSortedList(analytics.DeviceTypes, 10)

	return analytics
}

// mapToSortedList converts a map to a sorted list of key-value pairs
func (s *AnalyticsService) mapToSortedList(m map[string]int, limit int) []KeyValue {
	type kv struct {
		Key   string
		Value int
	}

	var kvList []kv
	for k, v := range m {
		kvList = append(kvList, kv{Key: k, Value: v})
	}

	// Sort by value in descending order
	sort.Slice(kvList, func(i, j int) bool {
		return kvList[i].Value > kvList[j].Value
	})

	// Convert to KeyValue slice and limit results
	var result []KeyValue
	for i, kv := range kvList {
		if i >= limit {
			break
		}
		result = append(result, KeyValue{
			Key:   kv.Key,
			Value: kv.Value,
		})
	}

	return result
}

// KeyValue represents a key-value pair for analytics
type KeyValue struct {
	Key   string `json:"key"`
	Value int    `json:"value"`
}

// FileStatus represents the status of a log file
type FileStatus struct {
	Path           string    `json:"path"`
	LastModified   time.Time `json:"last_modified"`
	LastSize       int64     `json:"last_size"`
	LastIndexed    time.Time `json:"last_indexed"`
	IsCompressed   bool      `json:"is_compressed"`
	HasTimeRange   bool      `json:"has_timerange"`
	TimeRangeStart time.Time `json:"timerange_start,omitzero"`
	TimeRangeEnd   time.Time `json:"timerange_end,omitzero"`
}

// IndexStatus represents comprehensive index status and statistics
type IndexStatus struct {
	DocumentCount uint64       `json:"document_count"`
	LogPaths      []string     `json:"log_paths"`
	LogPathsCount int          `json:"log_paths_count"`
	TotalFiles    int          `json:"total_files"`
	Files         []FileStatus `json:"files"`
}

// LogAnalytics represents comprehensive log analytics
type LogAnalytics struct {
	TotalRequests       int            `json:"total_requests"`
	UniqueIPCount       int            `json:"unique_ip_count"`
	AverageResponseTime float64        `json:"average_response_time"`
	ErrorRate           float64        `json:"error_rate"`
	UniqueIPs           map[string]int `json:"-"` // Internal use only
	StatusCodes         map[int]int    `json:"status_codes"`
	TopPaths            map[string]int `json:"-"` // Internal use only
	TopIPs              map[string]int `json:"-"` // Internal use only
	Countries           map[string]int `json:"-"` // Internal use only
	Browsers            map[string]int `json:"-"` // Internal use only
	OSes                map[string]int `json:"-"` // Internal use only
	DeviceTypes         map[string]int `json:"-"` // Internal use only
	TopPathsList        []KeyValue     `json:"top_paths"`
	TopIPsList          []KeyValue     `json:"top_ips"`
	CountriesList       []KeyValue     `json:"countries"`
	BrowsersList        []KeyValue     `json:"browsers"`
	OSList              []KeyValue     `json:"operating_systems"`
	DeviceTypesList     []KeyValue     `json:"device_types"`
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
	}
}
