package nginx_log

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// GetAllLogsWithIndex returns all cached log paths with their index status
func GetAllLogsWithIndex(filters ...func(*NginxLogWithIndex) bool) []*NginxLogWithIndex {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	result := make([]*NginxLogWithIndex, 0, len(logCache))

	// Get persistence manager for database index records
	persistence := NewPersistenceManager()
	persistenceIndexes, err := persistence.GetAllLogIndexes()
	if err != nil {
		logger.Warnf("Failed to get persistence indexes: %v", err)
		persistenceIndexes = []*model.NginxLogIndex{}
	}

	// Create a map of persistence indexes for quick lookup
	persistenceMap := make(map[string]*model.NginxLogIndex)
	for _, idx := range persistenceIndexes {
		persistenceMap[idx.Path] = idx
	}

	// Get analytics service for index status
	service := GetAnalyticsService()
	var indexStatus *IndexStatus
	if service != nil {
		status, err := service.GetIndexStatus()
		if err == nil {
			indexStatus = status
		}
	}

	// Create a map of indexed files for quick lookup
	indexedFiles := make(map[string]*FileStatus)
	if indexStatus != nil && indexStatus.Files != nil {
		for i := range indexStatus.Files {
			file := &indexStatus.Files[i]
			indexedFiles[file.Path] = file
		}
	}

	// Convert each log cache entry to log with index
	for _, cache := range logCache {
		logWithIndex := &NginxLogWithIndex{
			Path:         cache.Path,
			Type:         cache.Type,
			Name:         cache.Name,
			ConfigFile:   cache.ConfigFile,
			IndexStatus:  IndexStatusNotIndexed,
			IsCompressed: false,
			HasTimeRange: false,
		}

		// Check if this file is currently being indexed
		if IsFileIndexing(cache.Path) {
			logWithIndex.IndexStatus = IndexStatusIndexing
		}

		// Check persistence data first (more accurate)
		if persistenceIndex, ok := persistenceMap[cache.Path]; ok {
			// Set status based on persistence and current indexing state
			if logWithIndex.IndexStatus != IndexStatusIndexing {
				if !persistenceIndex.LastIndexed.IsZero() {
					logWithIndex.IndexStatus = IndexStatusIndexed
				}
			}
			
			// Use persistence data
			if !persistenceIndex.LastModified.IsZero() {
				logWithIndex.LastModified = persistenceIndex.LastModified.Unix()
			}
			logWithIndex.LastSize = persistenceIndex.LastSize
			if !persistenceIndex.LastIndexed.IsZero() {
				logWithIndex.LastIndexed = persistenceIndex.LastIndexed.Unix()
			}
			if persistenceIndex.IndexStartTime != nil {
				logWithIndex.IndexStartTime = persistenceIndex.IndexStartTime.Unix()
			}
			if persistenceIndex.IndexDuration != nil {
				logWithIndex.IndexDuration = *persistenceIndex.IndexDuration
			}
			if persistenceIndex.TimeRangeStart != nil {
				logWithIndex.TimeRangeStart = persistenceIndex.TimeRangeStart.Unix()
				logWithIndex.HasTimeRange = true
			}
			if persistenceIndex.TimeRangeEnd != nil {
				logWithIndex.TimeRangeEnd = persistenceIndex.TimeRangeEnd.Unix()
				logWithIndex.HasTimeRange = true
			}
			logWithIndex.DocumentCount = persistenceIndex.DocumentCount
		} else if fileStatus, ok := indexedFiles[cache.Path]; ok {
			// Fallback to old index status system
			if logWithIndex.IndexStatus != IndexStatusIndexing {
				logWithIndex.IndexStatus = IndexStatusIndexed
			}
			if fileStatus.LastModified != 0 {
				logWithIndex.LastModified = fileStatus.LastModified
			}
			logWithIndex.LastSize = fileStatus.LastSize
			if fileStatus.LastIndexed != 0 {
				logWithIndex.LastIndexed = fileStatus.LastIndexed
			}
			logWithIndex.IsCompressed = fileStatus.IsCompressed
			logWithIndex.HasTimeRange = fileStatus.HasTimeRange
			if fileStatus.TimeRangeStart != 0 {
				logWithIndex.TimeRangeStart = fileStatus.TimeRangeStart
			}
			if fileStatus.TimeRangeEnd != 0 {
				logWithIndex.TimeRangeEnd = fileStatus.TimeRangeEnd
			}
		}

		// Apply filters
		flag := true
		if len(filters) > 0 {
			for _, filter := range filters {
				if !filter(logWithIndex) {
					flag = false
					break
				}
			}
		}

		if flag {
			result = append(result, logWithIndex)
		}
	}

	return result
}