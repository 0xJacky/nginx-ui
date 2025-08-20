package nginx_log

import (
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/uozi-tech/cosy/logger"
)

// Helper functions for dashboard summary calculation
func (s *BleveStatsService) calculateAvgDailyUVFromStats(analytics *DashboardAnalytics) float64 {
	if len(analytics.DailyStats) == 0 {
		return 0.0
	}
	
	uvValues := make([]int, len(analytics.DailyStats))
	for i, daily := range analytics.DailyStats {
		uvValues[i] = daily.UV
	}
	return calculateAverage(uvValues)
}

func (s *BleveStatsService) calculateAvgDailyPVFromStats(analytics *DashboardAnalytics) float64 {
	if len(analytics.DailyStats) == 0 {
		return 0.0
	}
	
	pvValues := make([]int, len(analytics.DailyStats))
	for i, daily := range analytics.DailyStats {
		pvValues[i] = daily.PV
	}
	return calculateAverage(pvValues)
}

func (s *BleveStatsService) findPeakHourFromStats(analytics *DashboardAnalytics) int {
	if len(analytics.HourlyStats) == 0 {
		return 0
	}
	
	pvValues := make([]int, len(analytics.HourlyStats))
	for i, hourly := range analytics.HourlyStats {
		pvValues[i] = hourly.PV
	}
	
	_, maxIndex := findMax(pvValues)
	if maxIndex >= 0 && maxIndex < len(analytics.HourlyStats) {
		return analytics.HourlyStats[maxIndex].Hour
	}
	return 0
}

func (s *BleveStatsService) findPeakHourTrafficFromStats(analytics *DashboardAnalytics) int {
	if len(analytics.HourlyStats) == 0 {
		return 0
	}
	
	pvValues := make([]int, len(analytics.HourlyStats))
	for i, hourly := range analytics.HourlyStats {
		pvValues[i] = hourly.PV
	}
	
	maxTraffic, _ := findMax(pvValues)
	return maxTraffic
}

// extractTimestampAndIP extracts timestamp and IP from search hit
func (s *BleveStatsService) extractTimestampAndIP(hit *search.DocumentMatch) (*time.Time, string) {
	timestamp, ip, _ := s.extractTimestampIPAndPath(hit)
	return timestamp, ip
}

// extractTimestampIPAndPath extracts timestamp, IP, and file_path from search hit
func (s *BleveStatsService) extractTimestampIPAndPath(hit *search.DocumentMatch) (*time.Time, string, string) {
	var timestamp *time.Time
	var ip string
	var filePath string

	if timestampField, ok := hit.Fields["timestamp"]; ok {
		if timestampStr, ok := timestampField.(string); ok {
			if t, err := time.Parse(time.RFC3339, timestampStr); err == nil {
				timestamp = &t
			}
		}
	}

	if ipField, ok := hit.Fields["ip"]; ok {
		if ipStr, ok := ipField.(string); ok {
			ip = ipStr
		}
	}

	if filePathField, ok := hit.Fields["file_path"]; ok {
		if filePathStr, ok := filePathField.(string); ok {
			filePath = filePathStr
		}
	}

	return timestamp, ip, filePath
}

// GetTimeRangeFromBleve returns the available time range from Bleve index
func (s *BleveStatsService) GetTimeRangeFromBleve(logPath string) (start, end time.Time) {
	if s.indexer == nil {
		logger.Warn("BleveStatsService.GetTimeRangeFromBleve: indexer is nil")
		return time.Time{}, time.Time{}
	}

	if s.indexer.index == nil {
		logger.Warn("BleveStatsService.GetTimeRangeFromBleve: index is nil")
		return time.Time{}, time.Time{}
	}

	logger.Infof("BleveStatsService.GetTimeRangeFromBleve: Getting time range for log_path='%s'", logPath)

	var searchQuery query.Query = bleve.NewMatchAllQuery()

	// Add log path filter if specified
	if logPath != "" {
		// Use proper field-specific MatchQuery with keyword analyzer
		boolQuery := bleve.NewBooleanQuery()

		// Add base query (MatchAllQuery in this case)
		if searchQuery != nil {
			boolQuery.AddMust(searchQuery)
		}

		// Use MatchQuery with field specification for exact file_path matching
		filePathMatchQuery := bleve.NewMatchQuery(logPath)
		filePathMatchQuery.SetField("file_path") // Now this should work with TextFieldMapping + keyword analyzer
		boolQuery.AddMust(filePathMatchQuery)

		searchQuery = boolQuery
		logger.Infof("BleveStatsService.GetTimeRangeFromBleve: Using BooleanQuery with field-specific file_path MatchQuery for '%s'", logPath)
	}

	// Get earliest entry
	searchReq := bleve.NewSearchRequest(searchQuery)
	searchReq.Size = 1
	searchReq.Fields = []string{"timestamp"}
	searchReq.SortBy([]string{"timestamp"})

	logger.Debug("BleveStatsService.GetTimeRangeFromBleve: Searching for earliest entry")
	searchResult, err := s.indexer.index.Search(searchReq)
	if err != nil {
		logger.Errorf("BleveStatsService.GetTimeRangeFromBleve: Failed to search for earliest entry: %v", err)
		return time.Time{}, time.Time{}
	}
	if len(searchResult.Hits) == 0 {
		logger.Warn("BleveStatsService.GetTimeRangeFromBleve: No entries found for earliest search")
		return time.Time{}, time.Time{}
	}

	logger.Debugf("BleveStatsService.GetTimeRangeFromBleve: Found %d entries (total=%d)", len(searchResult.Hits), searchResult.Total)

	if timestampField, ok := searchResult.Hits[0].Fields["timestamp"]; ok {
		if timestampStr, ok := timestampField.(string); ok {
			if t, err := time.Parse(time.RFC3339, timestampStr); err == nil {
				start = t
			}
		}
	}

	// Get latest entry
	searchReq.SortBy([]string{"-timestamp"})
	searchResult, err = s.indexer.index.Search(searchReq)
	if err != nil || len(searchResult.Hits) == 0 {
		return start, start
	}

	if timestampField, ok := searchResult.Hits[0].Fields["timestamp"]; ok {
		if timestampStr, ok := timestampField.(string); ok {
			if t, err := time.Parse(time.RFC3339, timestampStr); err == nil {
				end = t
			}
		}
	}

	return start, end
}

// Global Bleve stats service instance
var bleveStatsService *BleveStatsService

// InitBleveStatsService initializes the global Bleve stats service
func InitBleveStatsService() {
	bleveStatsService = NewBleveStatsService()
	logger.Info("Bleve stats service initialized")
}

// GetBleveStatsService returns the global Bleve stats service instance
func GetBleveStatsService() *BleveStatsService {
	return bleveStatsService
}

// SetBleveStatsServiceIndexer sets the indexer for the global Bleve stats service
func SetBleveStatsServiceIndexer(indexer *LogIndexer) {
	if bleveStatsService != nil {
		bleveStatsService.SetIndexer(indexer)
	}
}