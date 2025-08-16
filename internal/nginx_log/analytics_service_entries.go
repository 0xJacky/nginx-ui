package nginx_log

import (
	"context"
	"fmt"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

// GetLogEntries retrieves log entries from a file
func (s *AnalyticsService) GetLogEntries(logPath string, limit int, tail bool) ([]*AccessLogEntry, error) {
	if logPath == "" {
		return nil, fmt.Errorf("log path is required")
	}

	// Validate log path
	if err := s.ValidateLogPath(logPath); err != nil {
		return nil, err
	}

	// Handle limit: 0 means no limit, negative values get default limit
	if limit < 0 {
		limit = 100
	}
	// Enforce maximum limit only if limit is not 0 (unlimited)
	if limit > 1000 && limit != 0 {
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

// GetIndexStatus returns comprehensive status and statistics about the indexer
func (s *AnalyticsService) GetIndexStatus() (*IndexStatus, error) {
	if s.indexer == nil {
		return nil, ErrIndexerNotAvailable
	}

	return s.indexer.GetIndexStatus()
}

// GetPreflightStatus returns the preflight status for log indexing
func (s *AnalyticsService) GetPreflightStatus(logPath string) (*PreflightResult, error) {
	var start, end time.Time
	var indexStatus string

	// Check real indexing status using IndexingStatusManager
	statusManager := GetIndexingStatusManager()
	isCurrentlyIndexing := statusManager.IsIndexing()

	if logPath != "" {
		// Validate log path exists
		if err := s.ValidateLogPath(logPath); err != nil {
			return nil, err
		}

		// Get time range from Bleve for specific log file
		start, end = s.GetTimeRangeFromSummaryStatsForPath(logPath)
		
		// Debug: Log the time range results
		logger.Debugf("File %s - start=%v, end=%v, start.IsZero()=%v, end.IsZero()=%v", 
			logPath, start, end, start.IsZero(), end.IsZero())
		
		// Check if this specific file is being indexed
		isFileIndexing := IsFileIndexing(logPath)
		logger.Debugf("File %s - IsFileIndexing=%v", logPath, isFileIndexing)
		
		if isFileIndexing {
			indexStatus = IndexStatusIndexing
			logger.Debugf("File %s is currently being indexed", logPath)
		} else if !start.IsZero() && !end.IsZero() {
			// File has been indexed and has data available in Bleve
			indexStatus = IndexStatusReady
			logger.Debugf("File %s is ready with data from %v to %v", logPath, start, end)
		} else {
			// Fallback: Check if file is actually indexed by querying index status
			logger.Debugf("Attempting fallback index status check for %s", logPath)
			indexStatusResult, err := s.GetIndexStatus()
			if err != nil {
				logger.Debugf("GetIndexStatus failed: %v", err)
			} else if indexStatusResult == nil {
				logger.Debugf("GetIndexStatus returned nil")
			} else {
				logger.Debugf("GetIndexStatus returned %d files", len(indexStatusResult.Files))
				// Look for this file in the index status
				found := false
				for _, file := range indexStatusResult.Files {
					if file.Path == logPath {
						found = true
						logger.Debugf("Found matching path %s, HasTimeRange=%v", logPath, file.HasTimeRange)
						if file.HasTimeRange && !file.TimeRangeStart.IsZero() && !file.TimeRangeEnd.IsZero() {
							// File is indexed with time range data
							start = file.TimeRangeStart
							end = file.TimeRangeEnd
							indexStatus = IndexStatusReady
							logger.Debugf("File %s found in index status with time range %v to %v", logPath, start, end)
							goto statusDetermined
						}
					}
				}
				if !found {
					logger.Debugf("File %s not found in index status", logPath)
				}
			}
			
			// File exists but either hasn't been indexed yet or has no data
			indexStatus = IndexStatusNotIndexed
			logger.Debugf("File %s has not been indexed or has no time range data", logPath)
		}
		statusDetermined:
	} else {
		// No log path available (default path not found)
		if isCurrentlyIndexing {
			indexStatus = IndexStatusIndexing
			logger.Debug("No specific log path, but indexing is currently in progress")
		} else {
			indexStatus = IndexStatusNotIndexed
			logger.Debug("No log path available and no indexing in progress")
		}
	}

	var startPtr, endPtr *time.Time
	if !start.IsZero() {
		startPtr = &start
	}
	if !end.IsZero() {
		endPtr = &end
	}

	// Data is available if we have time range data from Bleve or if currently indexing
	dataAvailable := (!start.IsZero() && !end.IsZero()) || indexStatus == IndexStatusIndexing

	result := &PreflightResult{
		StartTime:   startPtr,
		EndTime:     endPtr,
		Available:   dataAvailable,
		IndexStatus: indexStatus,
	}

	logger.Debugf("Preflight result: log_path=%s, available=%v, index_status=%s", 
		logPath, dataAvailable, indexStatus)

	return result, nil
}