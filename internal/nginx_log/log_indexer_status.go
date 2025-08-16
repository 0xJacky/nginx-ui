package nginx_log

import (
	"fmt"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/uozi-tech/cosy/logger"
)

// GetTimeRange returns the time range of indexed logs by querying Bleve directly
func (li *LogIndexer) GetTimeRange() (start, end time.Time) {
	logger.Infof("GetTimeRange called (querying Bleve for min/max timestamp)")

	// Find the minimum timestamp
	query := bleve.NewMatchAllQuery()
	searchRequestMin := bleve.NewSearchRequest(query)
	searchRequestMin.Size = 1
	searchRequestMin.SortBy([]string{"timestamp"}) // ascending is default
	searchRequestMin.Fields = []string{"timestamp"}

	searchResultMin, err := li.index.Search(searchRequestMin)
	if err != nil {
		logger.Warnf("Failed to query min time from Bleve: %v", err)
		return time.Time{}, time.Time{}
	}

	if searchResultMin.Total > 0 && len(searchResultMin.Hits) > 0 {
		if tsVal, ok := searchResultMin.Hits[0].Fields["timestamp"].(string); ok {
			start, _ = time.Parse(time.RFC3339, tsVal)
		}
	}

	// Find the maximum timestamp
	searchRequestMax := bleve.NewSearchRequest(query)
	searchRequestMax.Size = 1
	searchRequestMax.SortBy([]string{"-timestamp"}) // descending
	searchRequestMax.Fields = []string{"timestamp"}

	searchResultMax, err := li.index.Search(searchRequestMax)
	if err != nil {
		logger.Warnf("Failed to query max time from Bleve: %v", err)
		// Return start time even if max fails
		return start, time.Time{}
	}

	if searchResultMax.Total > 0 && len(searchResultMax.Hits) > 0 {
		if tsVal, ok := searchResultMax.Hits[0].Fields["timestamp"].(string); ok {
			end, _ = time.Parse(time.RFC3339, tsVal)
		}
	}

	logger.Infof("GetTimeRange result: start=%s, end=%s", start.Format(time.RFC3339), end.Format(time.RFC3339))
	return start, end
}

// GetTimeRangeForPath returns the time range for a specific log path using Bleve
func (li *LogIndexer) GetTimeRangeForPath(logPath string) (start, end time.Time) {
	logger.Infof("GetTimeRangeForPath called for %s (querying Bleve)", logPath)

	if logPath == "" {
		return li.GetTimeRange() // Fallback to general time range
	}

	// Create query for specific log path
	pathQuery := bleve.NewTermQuery(logPath)
	pathQuery.SetField("file_path")

	// Find minimum timestamp for this path
	searchRequestMin := bleve.NewSearchRequest(pathQuery)
	searchRequestMin.Size = 1
	searchRequestMin.SortBy([]string{"timestamp"})
	searchRequestMin.Fields = []string{"timestamp"}

	searchResultMin, err := li.index.Search(searchRequestMin)
	if err != nil {
		logger.Warnf("Failed to query min time for path %s: %v", logPath, err)
		return time.Time{}, time.Time{}
	}

	if searchResultMin.Total > 0 && len(searchResultMin.Hits) > 0 {
		if tsVal, ok := searchResultMin.Hits[0].Fields["timestamp"].(string); ok {
			start, _ = time.Parse(time.RFC3339, tsVal)
		}
	}

	// Find maximum timestamp for this path
	searchRequestMax := bleve.NewSearchRequest(pathQuery)
	searchRequestMax.Size = 1
	searchRequestMax.SortBy([]string{"-timestamp"})
	searchRequestMax.Fields = []string{"timestamp"}

	searchResultMax, err := li.index.Search(searchRequestMax)
	if err != nil {
		logger.Warnf("Failed to query max time for path %s: %v", logPath, err)
		return start, time.Time{}
	}

	if searchResultMax.Total > 0 && len(searchResultMax.Hits) > 0 {
		if tsVal, ok := searchResultMax.Hits[0].Fields["timestamp"].(string); ok {
			end, _ = time.Parse(time.RFC3339, tsVal)
		}
	}

	logger.Debugf("GetTimeRangeForPath result for %s: start=%s, end=%s", logPath, start.Format(time.RFC3339), end.Format(time.RFC3339))
	return start, end
}

// GetTimeRangeFromSummaryStatsForPath returns time range from Bleve stats service
func (li *LogIndexer) GetTimeRangeFromSummaryStatsForPath(logPath string) (start, end time.Time) {
	// Delegate to Bleve stats service
	bleveStatsService := GetBleveStatsService()
	if bleveStatsService == nil {
		return time.Time{}, time.Time{}
	}

	return bleveStatsService.GetTimeRangeFromBleve(logPath)
}

// GetIndexStatus returns comprehensive status information about the indexer
func (li *LogIndexer) GetIndexStatus() (*IndexStatus, error) {
	if li.index == nil {
		return nil, fmt.Errorf("index not available")
	}

	// Get document count
	docCount, err := li.index.DocCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get document count: %w", err)
	}

	// Get tracked log paths
	li.mu.RLock()
	logPaths := make([]string, 0, len(li.logPaths))
	files := make([]FileStatus, 0, len(li.logPaths))
	
	for path, info := range li.logPaths {
		logPaths = append(logPaths, path)
		
		fileStatus := FileStatus{
			Path:         path,
			LastModified: info.LastModified,
			LastSize:     info.LastSize,
			LastIndexed:  info.LastIndexed,
			IsCompressed: info.IsCompressed,
		}

		// Add time range information if available
		if info.TimeRange != nil {
			fileStatus.HasTimeRange = true
			fileStatus.TimeRangeStart = info.TimeRange.Start
			fileStatus.TimeRangeEnd = info.TimeRange.End
		}

		files = append(files, fileStatus)
	}
	li.mu.RUnlock()

	return &IndexStatus{
		DocumentCount: docCount,
		LogPaths:      logPaths,
		LogPathsCount: len(logPaths),
		TotalFiles:    len(files),
		Files:         files,
	}, nil
}

// IsIndexAvailable checks if the Bleve index is actually accessible for a given log path
func (li *LogIndexer) IsIndexAvailable(logPath string) bool {
	if li.index == nil {
		return false
	}

	// First check: try to get document count for the index
	docCount, err := li.index.DocCount()
	if err != nil {
		logger.Debugf("Index not accessible (DocCount failed): %v", err)
		return false
	}

	// If no documents at all, index exists but is empty
	if docCount == 0 {
		return false
	}

	// Second check: try a simple search for this specific log path
	pathQuery := bleve.NewTermQuery(logPath)
	pathQuery.SetField("file_path")
	searchRequest := bleve.NewSearchRequest(pathQuery)
	searchRequest.Size = 1

	result, err := li.index.Search(searchRequest)
	if err != nil {
		logger.Debugf("Index search failed for %s: %v", logPath, err)
		return false
	}

	// Return true if we found documents for this path
	return result.Total > 0
}