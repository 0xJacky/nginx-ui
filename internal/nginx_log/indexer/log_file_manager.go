package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// Legacy constants for backward compatibility - use IndexStatus enum in types.go instead

// NginxLogCache represents a cached log entry from nginx configuration
type NginxLogCache struct {
	Path       string `json:"path"`        // Path to the log file
	Type       string `json:"type"`        // Type of log: "access" or "error"
	Name       string `json:"name"`        // Name of the log file
	ConfigFile string `json:"config_file"` // Path to the configuration file that contains this log directive
}

// NginxLogWithIndex represents a log file with its index status information
type NginxLogWithIndex struct {
	Path           string `json:"path"`                       // Path to the log file
	Type           string `json:"type"`                       // Type of log: "access" or "error"
	Name           string `json:"name"`                       // Name of the log file
	ConfigFile     string `json:"config_file"`                // Path to the configuration file
	IndexStatus    string `json:"index_status"`               // Index status: indexed, indexing, not_indexed, queued, error
	LastModified   int64  `json:"last_modified,omitempty"`    // Unix timestamp of last modification time
	LastSize       int64  `json:"last_size,omitempty"`        // Last known size of the file
	LastIndexed    int64  `json:"last_indexed,omitempty"`     // Unix timestamp when the file was last indexed
	IndexStartTime int64  `json:"index_start_time,omitempty"` // Unix timestamp when the last indexing operation started
	IndexDuration  int64  `json:"index_duration,omitempty"`   // Duration of last indexing operation in milliseconds
	IsCompressed   bool   `json:"is_compressed"`              // Whether the file is compressed
	HasTimeRange   bool   `json:"has_timerange"`              // Whether time range is available
	TimeRangeStart int64  `json:"timerange_start,omitempty"`  // Unix timestamp of start of time range in the log
	TimeRangeEnd   int64  `json:"timerange_end,omitempty"`    // Unix timestamp of end of time range in the log
	DocumentCount  uint64 `json:"document_count,omitempty"`   // Number of indexed documents from this file
	// Enhanced status tracking fields
	ErrorMessage  string `json:"error_message,omitempty"`  // Error message if indexing failed
	ErrorTime     int64  `json:"error_time,omitempty"`     // Unix timestamp when error occurred
	RetryCount    int    `json:"retry_count,omitempty"`    // Number of retry attempts
	QueuePosition int    `json:"queue_position,omitempty"` // Position in indexing queue
}

// LogFileManager manages nginx log file discovery and index status
type LogFileManager struct {
	logCache       map[string]*NginxLogCache
	cacheMutex     sync.RWMutex
	persistence    *PersistenceManager
	indexingStatus map[string]bool
	indexingMutex  sync.RWMutex
}

// NewLogFileManager creates a new log file manager
func NewLogFileManager() *LogFileManager {
	return &LogFileManager{
		logCache:       make(map[string]*NginxLogCache),
		persistence:    NewPersistenceManager(DefaultIncrementalConfig()),
		indexingStatus: make(map[string]bool),
	}
}

// AddLogPath adds a log path to the log cache with the source config file
func (lm *LogFileManager) AddLogPath(path, logType, name, configFile string) {
	lm.cacheMutex.Lock()
	defer lm.cacheMutex.Unlock()

	lm.logCache[path] = &NginxLogCache{
		Path:       path,
		Type:       logType,
		Name:       name,
		ConfigFile: configFile,
	}
}

// RemoveLogPathsFromConfig removes all log paths associated with a specific config file
func (lm *LogFileManager) RemoveLogPathsFromConfig(configFile string) {
	lm.cacheMutex.Lock()
	defer lm.cacheMutex.Unlock()

	for path, logEntry := range lm.logCache {
		if logEntry.ConfigFile == configFile {
			delete(lm.logCache, path)
		}
	}
}

// GetAllLogPaths returns all cached log paths, optionally filtered
func (lm *LogFileManager) GetAllLogPaths(filters ...func(*NginxLogCache) bool) []*NginxLogCache {
	lm.cacheMutex.RLock()
	defer lm.cacheMutex.RUnlock()

	var logs []*NginxLogCache

	for _, logEntry := range lm.logCache {
		// Apply all filters
		include := true
		for _, filter := range filters {
			if !filter(logEntry) {
				include = false
				break
			}
		}

		if include {
			// Create a copy to avoid race conditions
			logCopy := *logEntry
			logs = append(logs, &logCopy)
		}
	}

	return logs
}

// SetIndexingStatus sets the indexing status for a specific file path
func (lm *LogFileManager) SetIndexingStatus(path string, isIndexing bool) {
	lm.indexingMutex.Lock()
	defer lm.indexingMutex.Unlock()

	if isIndexing {
		lm.indexingStatus[path] = true
	} else {
		delete(lm.indexingStatus, path)
	}
}

// GetIndexingFiles returns a list of files currently being indexed
func (lm *LogFileManager) GetIndexingFiles() []string {
	lm.indexingMutex.RLock()
	defer lm.indexingMutex.RUnlock()

	var files []string
	for path := range lm.indexingStatus {
		files = append(files, path)
	}

	return files
}

// getBaseLogName determines the base log file name for grouping rotated files
func getBaseLogName(filePath string) string {
	dir := filepath.Dir(filePath)
	filename := filepath.Base(filePath)

	// Remove compression extensions first
	filename = strings.TrimSuffix(filename, ".gz")
	filename = strings.TrimSuffix(filename, ".bz2")

	// Handle numbered rotation (access.log.1, access.log.2, etc.)
	if match := regexp.MustCompile(`^(.+)\.(\d+)$`).FindStringSubmatch(filename); len(match) > 1 {
		baseFilename := match[1]
		return filepath.Join(dir, baseFilename)
	}

	// Handle date rotation suffixes
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

	// If it already looks like a base log file, return as-is
	return filePath
}

// GetAllLogsWithIndexGrouped returns logs grouped by their base name (e.g., access.log includes access.log.1, access.log.2.gz etc.)
func (lm *LogFileManager) GetAllLogsWithIndexGrouped(filters ...func(*NginxLogWithIndex) bool) []*NginxLogWithIndex {
	lm.cacheMutex.RLock()
	defer lm.cacheMutex.RUnlock()

	// Get all logs from both cache (config files) and persistence (indexed files)
	allLogsMap := make(map[string]*NginxLogWithIndex)

	// First, get logs from the cache (these are from nginx config)
	for _, cache := range lm.logCache {
		logWithIndex := &NginxLogWithIndex{
			Path:         cache.Path,
			Type:         cache.Type,
			Name:         cache.Name,
			ConfigFile:   cache.ConfigFile,
			IndexStatus:  string(IndexStatusNotIndexed),
			IsCompressed: false,
			HasTimeRange: false,
		}
		allLogsMap[cache.Path] = logWithIndex
	}

	// Get persistence indexes and update status
	persistenceIndexes, err := lm.persistence.GetAllLogIndexes()
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
				Path:        idx.Path,
				Type:        logType,
				Name:        filepath.Base(idx.Path),
				ConfigFile:  "",
				IndexStatus: string(IndexStatusNotIndexed),
			}
			allLogsMap[idx.Path] = logWithIndex
		}

		// Update index status from persistence data
		logWithIndex := allLogsMap[idx.Path]
		logWithIndex.LastModified = idx.LastModified.Unix()
		logWithIndex.LastSize = idx.LastSize
		logWithIndex.LastIndexed = idx.LastIndexed.Unix()
		if idx.IndexStartTime != nil {
			logWithIndex.IndexStartTime = idx.IndexStartTime.Unix()
		}
		if idx.IndexDuration != nil {
			logWithIndex.IndexDuration = *idx.IndexDuration
		}
		logWithIndex.DocumentCount = idx.DocumentCount

		// Set queue position if available
		logWithIndex.QueuePosition = idx.QueuePosition

		// Set error message if available
		logWithIndex.ErrorMessage = idx.ErrorMessage
		if idx.ErrorTime != nil {
			logWithIndex.ErrorTime = idx.ErrorTime.Unix()
		}
		logWithIndex.RetryCount = idx.RetryCount

		// Use the index status from the database if it's set
		if idx.IndexStatus != "" {
			logWithIndex.IndexStatus = idx.IndexStatus
		} else {
			// Fallback to determining status if not set in DB
			lm.indexingMutex.RLock()
			isIndexing := lm.indexingStatus[idx.Path]
			lm.indexingMutex.RUnlock()

			if isIndexing {
				logWithIndex.IndexStatus = string(IndexStatusIndexing)
			} else if !idx.LastIndexed.IsZero() {
				// If file has been indexed (regardless of document count), it's indexed
				logWithIndex.IndexStatus = string(IndexStatusIndexed)
			}
		}

		// Set time range if available
		if idx.TimeRangeStart != nil && idx.TimeRangeEnd != nil && !idx.TimeRangeStart.IsZero() && !idx.TimeRangeEnd.IsZero() {
			logWithIndex.HasTimeRange = true
			logWithIndex.TimeRangeStart = idx.TimeRangeStart.Unix()
			logWithIndex.TimeRangeEnd = idx.TimeRangeEnd.Unix()
		}

		logWithIndex.IsCompressed = strings.HasSuffix(idx.Path, ".gz") || strings.HasSuffix(idx.Path, ".bz2")
	}

	// Convert to slice and apply filters
	var logs []*NginxLogWithIndex
	for _, log := range allLogsMap {
		// Apply all filters
		include := true
		for _, filter := range filters {
			if !filter(log) {
				include = false
				break
			}
		}

		if include {
			logs = append(logs, log)
		}
	}

	// Group by base log name with stable aggregation
	groupedMap := make(map[string]*NginxLogWithIndex)

	// Sort logs by path first to ensure consistent processing order
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Path < logs[j].Path
	})

	for _, log := range logs {
		baseLogName := getBaseLogName(log.Path)

		if existing, exists := groupedMap[baseLogName]; exists {
			// Check if current log is a main log path record (already aggregated)
			// or if existing record is a main log path record
			logIsMainPath := (log.Path == baseLogName)
			existingIsMainPath := (existing.Path == baseLogName)

			if logIsMainPath && !existingIsMainPath {
				// Current log is the main aggregated record, replace existing
				groupedLog := *log
				groupedLog.Path = baseLogName
				groupedLog.Name = filepath.Base(baseLogName)
				groupedMap[baseLogName] = &groupedLog
			} else if !logIsMainPath && existingIsMainPath {
				// Existing is main record, keep it, don't accumulate
				// Only update status if needed
				if log.IndexStatus == string(IndexStatusIndexing) {
					existing.IndexStatus = string(IndexStatusIndexing)
				}
			} else if !logIsMainPath && !existingIsMainPath {
				// Both are individual files, accumulate normally
				if log.LastIndexed > existing.LastIndexed {
					existing.LastModified = log.LastModified
					existing.LastIndexed = log.LastIndexed
					existing.IndexStartTime = log.IndexStartTime
					existing.IndexDuration = log.IndexDuration
				}

				existing.DocumentCount += log.DocumentCount
				existing.LastSize += log.LastSize

				// Update status with priority: indexing > queued > indexed > error > not_indexed
				if log.IndexStatus == string(IndexStatusIndexing) {
					existing.IndexStatus = string(IndexStatusIndexing)
				} else if log.IndexStatus == string(IndexStatusQueued) &&
					existing.IndexStatus != string(IndexStatusIndexing) {
					existing.IndexStatus = string(IndexStatusQueued)
					// Keep the queue position from the queued log
					if log.QueuePosition > 0 {
						existing.QueuePosition = log.QueuePosition
					}
				} else if log.IndexStatus == string(IndexStatusIndexed) &&
					existing.IndexStatus != string(IndexStatusIndexing) &&
					existing.IndexStatus != string(IndexStatusQueued) {
					existing.IndexStatus = string(IndexStatusIndexed)
				} else if log.IndexStatus == string(IndexStatusError) &&
					existing.IndexStatus != string(IndexStatusIndexing) &&
					existing.IndexStatus != string(IndexStatusQueued) &&
					existing.IndexStatus != string(IndexStatusIndexed) {
					existing.IndexStatus = string(IndexStatusError)
					existing.ErrorMessage = log.ErrorMessage
					existing.ErrorTime = log.ErrorTime
				}

				if log.HasTimeRange {
					if !existing.HasTimeRange {
						existing.HasTimeRange = true
						existing.TimeRangeStart = log.TimeRangeStart
						existing.TimeRangeEnd = log.TimeRangeEnd
					} else {
						if log.TimeRangeStart > 0 && (existing.TimeRangeStart == 0 || log.TimeRangeStart < existing.TimeRangeStart) {
							existing.TimeRangeStart = log.TimeRangeStart
						}
						if log.TimeRangeEnd > existing.TimeRangeEnd {
							existing.TimeRangeEnd = log.TimeRangeEnd
						}
					}
				}
			} else if logIsMainPath && existingIsMainPath {
				// If both are main paths, use the one with more recent LastIndexed
				if log.LastIndexed > existing.LastIndexed {
					groupedLog := *log
					groupedLog.Path = baseLogName
					groupedLog.Name = filepath.Base(baseLogName)
					groupedMap[baseLogName] = &groupedLog
				}
			}
		} else {
			// Create new entry with base log name as path for grouping
			groupedLog := *log
			groupedLog.Path = baseLogName
			groupedLog.Name = filepath.Base(baseLogName)
			// Preserve queue position and error info for the grouped log
			groupedLog.QueuePosition = log.QueuePosition
			groupedLog.ErrorMessage = log.ErrorMessage
			groupedLog.ErrorTime = log.ErrorTime
			groupedLog.RetryCount = log.RetryCount
			groupedMap[baseLogName] = &groupedLog
		}
	}

	// Convert map to slice with consistent ordering
	var result []*NginxLogWithIndex

	// Create a sorted list of keys to ensure consistent order
	var keys []string
	for key := range groupedMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Build result in consistent order
	for _, key := range keys {
		result = append(result, groupedMap[key])
	}

	// --- START DIAGNOSTIC LOGGING ---
	logger.Debugf("===== FINAL GROUPED LIST =====")
	for _, fLog := range result {
		logger.Debugf("Final Group: Path=%s, DocCount=%d, Status=%s", fLog.Path, fLog.DocumentCount, fLog.IndexStatus)
	}
	logger.Debugf("===============================")
	// --- END DIAGNOSTIC LOGGING ---

	return result
}

// SaveIndexMetadata saves the metadata for a log group after an indexing operation.
// It creates a new record for the base log path.
func (lm *LogFileManager) SaveIndexMetadata(basePath string, documentCount uint64, startTime time.Time, duration time.Duration, minTime *time.Time, maxTime *time.Time) error {
	// We want to save the metadata against the base path (the "log group").
	// We get or create a record for this specific path.
	logIndex, err := lm.persistence.GetLogIndex(basePath)
	if err != nil {
		// If the error is anything other than "not found", it's a real problem.
		// GetLogIndex is designed to return a new object if not found, so this should be rare.
		return fmt.Errorf("could not get or create log index for '%s': %w", basePath, err)
	}

	// Get file stats to update LastModified and LastSize
	if fileInfo, err := os.Stat(basePath); err == nil {
		logIndex.LastModified = fileInfo.ModTime()
		logIndex.LastSize = fileInfo.Size()
	}

	// Update the record with the new metadata
	logIndex.DocumentCount = documentCount
	logIndex.LastIndexed = time.Now()
	logIndex.IndexStartTime = &startTime
	durationMs := duration.Milliseconds()
	logIndex.IndexDuration = &durationMs

	// Merge time ranges: preserve existing historical range and expand if necessary
	// This prevents incremental indexing from losing historical time range data
	if minTime != nil {
		if logIndex.TimeRangeStart == nil || minTime.Before(*logIndex.TimeRangeStart) {
			logIndex.TimeRangeStart = minTime
		}
	}
	if maxTime != nil {
		if logIndex.TimeRangeEnd == nil || maxTime.After(*logIndex.TimeRangeEnd) {
			logIndex.TimeRangeEnd = maxTime
		}
	}

	// Save the updated record to the database
	return lm.persistence.SaveLogIndex(logIndex)
}

// DeleteIndexMetadataByGroup deletes all database records for a given log group.
func (lm *LogFileManager) DeleteIndexMetadataByGroup(basePath string) error {
	// The basePath is the main log path for the group.
	return lm.persistence.DeleteLogIndexesByGroup(basePath)
}

// DeleteAllIndexMetadata deletes all index metadata from the database.
func (lm *LogFileManager) DeleteAllIndexMetadata() error {
	return lm.persistence.DeleteAllLogIndexes()
}

// GetLogByPath returns the full NginxLogWithIndex struct for a given base path.
func (lm *LogFileManager) GetLogByPath(basePath string) (*NginxLogWithIndex, error) {
	// This is not the most efficient way, but it's reliable.
	// It ensures we get the same grouped and aggregated data the UI sees.
	allLogs := lm.GetAllLogsWithIndexGrouped()
	for _, log := range allLogs {
		if log.Path == basePath {
			return log, nil
		}
	}
	return nil, fmt.Errorf("log group with base path not found: %s", basePath)
}

// GetFilePathsForGroup returns all physical file paths for a given log group base path.
func (lm *LogFileManager) GetFilePathsForGroup(basePath string) ([]string, error) {
	// Query the database for all log indexes with matching main_log_path
	logIndexes, err := lm.persistence.GetLogIndexesByGroup(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get log indexes for group %s: %w", basePath, err)
	}

	// Extract file paths from the database records
	filePaths := make([]string, 0, len(logIndexes))
	for _, logIndex := range logIndexes {
		filePaths = append(filePaths, logIndex.Path)
	}

	return filePaths, nil
}

// GetPersistence returns the persistence manager for advanced operations
func (lm *LogFileManager) GetPersistence() *PersistenceManager {
	return lm.persistence
}

// GetAllLogsWithIndex returns all cached log paths with their index status (non-grouped)
func (lm *LogFileManager) GetAllLogsWithIndex(filters ...func(*NginxLogWithIndex) bool) []*NginxLogWithIndex {
	lm.cacheMutex.RLock()
	defer lm.cacheMutex.RUnlock()

	result := make([]*NginxLogWithIndex, 0, len(lm.logCache))

	// Get persistence indexes
	persistenceIndexes, err := lm.persistence.GetAllLogIndexes()
	if err != nil {
		logger.Warnf("Failed to get persistence indexes: %v", err)
		persistenceIndexes = []*model.NginxLogIndex{}
	}

	// Create a map of persistence indexes for quick lookup
	persistenceMap := make(map[string]*model.NginxLogIndex)
	for _, idx := range persistenceIndexes {
		persistenceMap[idx.Path] = idx
	}

	// Process cached logs (from nginx config)
	for _, cache := range lm.logCache {
		logWithIndex := &NginxLogWithIndex{
			Path:         cache.Path,
			Type:         cache.Type,
			Name:         cache.Name,
			ConfigFile:   cache.ConfigFile,
			IndexStatus:  string(IndexStatusNotIndexed),
			IsCompressed: strings.HasSuffix(cache.Path, ".gz") || strings.HasSuffix(cache.Path, ".bz2"),
		}

		// Update with persistence data if available
		if idx, exists := persistenceMap[cache.Path]; exists {
			logWithIndex.LastModified = idx.LastModified.Unix()
			logWithIndex.LastSize = idx.LastSize
			logWithIndex.LastIndexed = idx.LastIndexed.Unix()
			if idx.IndexStartTime != nil {
				logWithIndex.IndexStartTime = idx.IndexStartTime.Unix()
			}
			if idx.IndexDuration != nil {
				logWithIndex.IndexDuration = *idx.IndexDuration
			}
			logWithIndex.DocumentCount = idx.DocumentCount

			// Determine status
			lm.indexingMutex.RLock()
			isIndexing := lm.indexingStatus[cache.Path]
			lm.indexingMutex.RUnlock()

			if isIndexing {
				logWithIndex.IndexStatus = string(IndexStatusIndexing)
			} else if !idx.LastIndexed.IsZero() {
				// If file has been indexed (regardless of document count), it's indexed
				logWithIndex.IndexStatus = string(IndexStatusIndexed)
			}

			// Set time range if available
			if idx.TimeRangeStart != nil && idx.TimeRangeEnd != nil && !idx.TimeRangeStart.IsZero() && !idx.TimeRangeEnd.IsZero() {
				logWithIndex.HasTimeRange = true
				logWithIndex.TimeRangeStart = idx.TimeRangeStart.Unix()
				logWithIndex.TimeRangeEnd = idx.TimeRangeEnd.Unix()
			}
		}

		// Apply filters
		include := true
		for _, filter := range filters {
			if !filter(logWithIndex) {
				include = false
				break
			}
		}

		if include {
			result = append(result, logWithIndex)
		}
	}

	return result
}
