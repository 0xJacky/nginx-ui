package nginx_log

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// GetAllLogsWithIndexGrouped returns logs grouped by their base name (e.g., access.log includes access.log.1, access.log.2.gz etc.)
func GetAllLogsWithIndexGrouped(filters ...func(*NginxLogWithIndex) bool) []*NginxLogWithIndex {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	// Get all logs from both cache (config files) and persistence (indexed files)
	allLogsMap := make(map[string]*NginxLogWithIndex)
	
	// First, get logs from the cache (these are from nginx config)
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
		allLogsMap[cache.Path] = logWithIndex
	}

	// Get persistence manager for database index records
	persistence := NewPersistenceManager()
	persistenceIndexes, err := persistence.GetAllLogIndexes()
	if err != nil {
		logger.Warnf("Failed to get persistence indexes: %v", err)
		persistenceIndexes = []*model.NginxLogIndex{}
	}

	// Add all indexed files from persistence (including rotated files)
	for _, idx := range persistenceIndexes {
		if _, exists := allLogsMap[idx.Path]; !exists {
			// This is a rotated file not in config cache, create entry for it
			logType := "access"
			if strings.Contains(idx.Path, "error") {
				logType = "error"
			}
			
			logWithIndex := &NginxLogWithIndex{
				Path:         idx.Path,
				Type:         logType,
				Name:         filepath.Base(idx.Path),
				ConfigFile:   "", // Rotated files don't have config
				IndexStatus:  IndexStatusNotIndexed,
				IsCompressed: strings.HasSuffix(idx.Path, ".gz") || strings.HasSuffix(idx.Path, ".bz2"),
				HasTimeRange: false,
			}
			allLogsMap[idx.Path] = logWithIndex
		}
	}

	// Now populate index information for all logs
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

	// Update index information for all logs
	for _, log := range allLogsMap {
		// Check if this file is currently being indexed
		if IsFileIndexing(log.Path) {
			log.IndexStatus = IndexStatusIndexing
		}

		// Check persistence data first (more accurate)
		if persistenceIndex, ok := persistenceMap[log.Path]; ok {
			// Set status based on persistence and current indexing state
			if log.IndexStatus != IndexStatusIndexing {
				if !persistenceIndex.LastIndexed.IsZero() {
					log.IndexStatus = IndexStatusIndexed
				}
			}
			
			// Use persistence data
			if !persistenceIndex.LastModified.IsZero() {
				log.LastModified = persistenceIndex.LastModified.Unix()
			}
			log.LastSize = persistenceIndex.LastSize
			if !persistenceIndex.LastIndexed.IsZero() {
				log.LastIndexed = persistenceIndex.LastIndexed.Unix()
			}
			if persistenceIndex.IndexStartTime != nil {
				log.IndexStartTime = persistenceIndex.IndexStartTime.Unix()
			}
			if persistenceIndex.IndexDuration != nil {
				log.IndexDuration = *persistenceIndex.IndexDuration
			}
			if persistenceIndex.TimeRangeStart != nil {
				log.TimeRangeStart = persistenceIndex.TimeRangeStart.Unix()
				log.HasTimeRange = true
			}
			if persistenceIndex.TimeRangeEnd != nil {
				log.TimeRangeEnd = persistenceIndex.TimeRangeEnd.Unix()
				log.HasTimeRange = true
			}
			log.DocumentCount = persistenceIndex.DocumentCount
		} else if fileStatus, ok := indexedFiles[log.Path]; ok {
			// Fallback to old index status system
			if log.IndexStatus != IndexStatusIndexing {
				log.IndexStatus = IndexStatusIndexed
			}
			if fileStatus.LastModified != 0 {
				log.LastModified = fileStatus.LastModified
			}
			log.LastSize = fileStatus.LastSize
			if fileStatus.LastIndexed != 0 {
				log.LastIndexed = fileStatus.LastIndexed
			}
			log.IsCompressed = fileStatus.IsCompressed
			log.HasTimeRange = fileStatus.HasTimeRange
			if fileStatus.TimeRangeStart != 0 {
				log.TimeRangeStart = fileStatus.TimeRangeStart
			}
			if fileStatus.TimeRangeEnd != 0 {
				log.TimeRangeEnd = fileStatus.TimeRangeEnd
			}
		}
	}

	// Convert map to slice
	allLogs := make([]*NginxLogWithIndex, 0, len(allLogsMap))
	for _, log := range allLogsMap {
		allLogs = append(allLogs, log)
	}
	
	// Group logs by their base log name
	logGroups := make(map[string][]*NginxLogWithIndex)
	for _, log := range allLogs {
		baseLogName := getBaseLogName(log.Path)
		logGroups[baseLogName] = append(logGroups[baseLogName], log)
	}
	
	result := make([]*NginxLogWithIndex, 0, len(logGroups))
	
	// Process each group
	for baseLogName, group := range logGroups {
		// Find the main log file (the one without rotation suffix)
		var mainLog *NginxLogWithIndex
		for _, log := range group {
			if isMainLogFile(log.Path, baseLogName) {
				mainLog = log
				break
			}
		}
		
		// If no main log file found, create one based on the base name
		if mainLog == nil {
			// Create a virtual main log based on the group's characteristics
			// Use the first log in the group as a template
			template := group[0]
			mainLog = &NginxLogWithIndex{
				Path:         baseLogName,
				Type:         template.Type,
				Name:         filepath.Base(baseLogName),
				ConfigFile:   template.ConfigFile,
				IndexStatus:  IndexStatusNotIndexed,
				IsCompressed: false,
				HasTimeRange: false,
			}
		}
		
		// Aggregate statistics from all files in the group
		aggregateLogGroupStats(mainLog, group)
		
		// Apply filters
		flag := true
		if len(filters) > 0 {
			for _, filter := range filters {
				if !filter(mainLog) {
					flag = false
					break
				}
			}
		}

		if flag {
			result = append(result, mainLog)
		}
	}
	
	return result
}

// getBaseLogName extracts the base log name from a rotated log file path
// Examples:
//   /var/log/nginx/access.log.1 -> /var/log/nginx/access.log
//   /var/log/nginx/access.log.10.gz -> /var/log/nginx/access.log
//   /var/log/nginx/access.20231201.gz -> /var/log/nginx/access.log
func getBaseLogName(logPath string) string {
	dir := filepath.Dir(logPath)
	filename := filepath.Base(logPath)
	
	// Remove .gz compression suffix if present
	if strings.HasSuffix(filename, ".gz") {
		filename = strings.TrimSuffix(filename, ".gz")
	}
	
	// Handle numbered rotation (access.log.1, access.log.2, etc.)
	// Use a more specific pattern to avoid matching date patterns like "20231201"
	if match := regexp.MustCompile(`^(.+)\.(\d{1,3})$`).FindStringSubmatch(filename); len(match) > 1 {
		// Only match if the number is reasonable for rotation (1-999)
		baseFilename := match[1]
		return filepath.Join(dir, baseFilename)
	}
	
	// Handle date-based rotation (access.20231201, access.2023-12-01, etc.)
	// Check if filename itself contains date patterns that we should strip
	// Example: access.2023-12-01 -> access.log, access.20231201 -> access.log
	parts := strings.Split(filename, ".")
	if len(parts) >= 2 {
		lastPart := parts[len(parts)-1]
		if isDatePattern(lastPart) {
			baseFilename := strings.Join(parts[:len(parts)-1], ".")
			// If the base doesn't end with .log, add it
			if !strings.HasSuffix(baseFilename, ".log") {
				baseFilename += ".log"
			}
			return filepath.Join(dir, baseFilename)
		}
	}
	
	// No rotation pattern found, return as-is
	return logPath
}

// isMainLogFile checks if the given path is the main log file (no rotation suffix)
func isMainLogFile(logPath, baseLogName string) bool {
	return logPath == baseLogName
}

// aggregateLogGroupStats aggregates statistics from all files in a log group
func aggregateLogGroupStats(aggregatedLog *NginxLogWithIndex, group []*NginxLogWithIndex) {
	var totalSize int64
	var totalDocuments uint64
	var earliestTimeStart *time.Time
	var latestTimeEnd *time.Time
	var mostRecentIndexed *time.Time
	var indexingInProgress bool
	var hasIndexedFiles bool
	var earliestIndexStartTime *time.Time
	var totalIndexDuration *int64
	
	for _, log := range group {
		// Aggregate file sizes
		totalSize += log.LastSize
		
		// Aggregate document counts
		totalDocuments += log.DocumentCount
		
		// Check for indexing status
		if log.IndexStatus == IndexStatusIndexing {
			indexingInProgress = true
		} else if log.IndexStatus == IndexStatusIndexed {
			hasIndexedFiles = true
		}
		
		// Find the most recent indexed time
		if log.LastIndexed != 0 {
			indexedTime := time.Unix(log.LastIndexed, 0)
			if mostRecentIndexed == nil || indexedTime.After(*mostRecentIndexed) {
				mostRecentIndexed = &indexedTime
			}
		}
		
		// Aggregate time ranges
		if log.TimeRangeStart != 0 {
			startTime := time.Unix(log.TimeRangeStart, 0)
			if earliestTimeStart == nil || startTime.Before(*earliestTimeStart) {
				earliestTimeStart = &startTime
			}
		}
		
		if log.TimeRangeEnd != 0 {
			endTime := time.Unix(log.TimeRangeEnd, 0)
			if latestTimeEnd == nil || endTime.After(*latestTimeEnd) {
				latestTimeEnd = &endTime
			}
		}
		
		// Use properties from the most recent file
		if log.LastModified != 0 && (aggregatedLog.LastModified == 0 || log.LastModified > aggregatedLog.LastModified) {
			aggregatedLog.LastModified = log.LastModified
		}
		
		// Find the EARLIEST IndexStartTime for the log group (when the group indexing started)
		if log.IndexStartTime != 0 {
			startTime := time.Unix(log.IndexStartTime, 0)
			if earliestIndexStartTime == nil || startTime.Before(*earliestIndexStartTime) {
				earliestIndexStartTime = &startTime
			}
		}
		
		// Sum up individual file durations to get total group duration
		if log.IndexDuration != 0 {
			if totalIndexDuration == nil {
				totalIndexDuration = new(int64)
			}
			*totalIndexDuration += log.IndexDuration
		}
	}
	
	// Set aggregated values
	if earliestIndexStartTime != nil {
		aggregatedLog.IndexStartTime = earliestIndexStartTime.Unix()
	}
	aggregatedLog.LastSize = totalSize
	aggregatedLog.DocumentCount = totalDocuments
	if mostRecentIndexed != nil {
		aggregatedLog.LastIndexed = mostRecentIndexed.Unix()
	}
	if totalIndexDuration != nil {
		aggregatedLog.IndexDuration = *totalIndexDuration
	}
	
	// Set index status based on group status
	if indexingInProgress {
		aggregatedLog.IndexStatus = IndexStatusIndexing
	} else if hasIndexedFiles {
		aggregatedLog.IndexStatus = IndexStatusIndexed
	} else {
		aggregatedLog.IndexStatus = IndexStatusNotIndexed
	}
	
	// Set time range
	if earliestTimeStart != nil && latestTimeEnd != nil {
		aggregatedLog.TimeRangeStart = earliestTimeStart.Unix()
		aggregatedLog.TimeRangeEnd = latestTimeEnd.Unix()
		aggregatedLog.HasTimeRange = true
	}
}