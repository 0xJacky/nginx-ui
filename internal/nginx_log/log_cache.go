package nginx_log

import (
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// IndexStatus constants
const (
	IndexStatusIndexed    = "indexed"
	IndexStatusIndexing   = "indexing" 
	IndexStatusNotIndexed = "not_indexed"
)

// NginxLogCache represents a cached log entry from nginx configuration
type NginxLogCache struct {
	Path       string `json:"path"`        // Path to the log file
	Type       string `json:"type"`        // Type of log: "access" or "error"
	Name       string `json:"name"`        // Name of the log file
	ConfigFile string `json:"config_file"` // Path to the configuration file that contains this log directive
}

// NginxLogWithIndex represents a log file with its index status information
type NginxLogWithIndex struct {
	Path           string     `json:"path"`                      // Path to the log file
	Type           string     `json:"type"`                      // Type of log: "access" or "error"
	Name           string     `json:"name"`                      // Name of the log file
	ConfigFile     string     `json:"config_file"`               // Path to the configuration file
	IndexStatus    string     `json:"index_status"`              // Index status: indexed, indexing, not_indexed
	LastModified   *time.Time `json:"last_modified,omitempty"`   // Last modification time of the file
	LastSize       int64      `json:"last_size,omitempty"`       // Last known size of the file
	LastIndexed    *time.Time `json:"last_indexed,omitempty"`    // When the file was last indexed
	IsCompressed   bool       `json:"is_compressed"`             // Whether the file is compressed
	HasTimeRange   bool       `json:"has_timerange"`             // Whether time range is available
	TimeRangeStart *time.Time `json:"timerange_start,omitempty"` // Start of time range in the log
	TimeRangeEnd   *time.Time `json:"timerange_end,omitempty"`   // End of time range in the log
	DocumentCount  uint64     `json:"document_count,omitempty"`  // Number of indexed documents from this file
}

var (
	// logCache is the map to store all found log files
	logCache   = make(map[string]*NginxLogCache)
	cacheMutex sync.RWMutex

	// indexingFiles tracks which files are currently being indexed
	indexingFiles = make(map[string]bool)
	indexingMutex sync.RWMutex
)

// SetIndexingStatus updates the indexing status for a file
func SetIndexingStatus(path string, isIndexing bool) {
	indexingMutex.Lock()
	defer indexingMutex.Unlock()

	if isIndexing {
		indexingFiles[path] = true
	} else {
		delete(indexingFiles, path)
	}
}

// IsFileIndexing checks if a file is currently being indexed
func IsFileIndexing(path string) bool {
	indexingMutex.RLock()
	defer indexingMutex.RUnlock()

	return indexingFiles[path]
}

// GetIndexingFiles returns all files currently being indexed
func GetIndexingFiles() []string {
	indexingMutex.RLock()
	defer indexingMutex.RUnlock()

	files := make([]string, 0, len(indexingFiles))
	for path := range indexingFiles {
		files = append(files, path)
	}
	return files
}

// AddLogPath adds a log path to the log cache with the source config file
func AddLogPath(path, logType, name, configFile string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	logCache[path] = &NginxLogCache{
		Path:       path,
		Type:       logType,
		Name:       name,
		ConfigFile: configFile,
	}
}

// RemoveLogPathsFromConfig removes all log paths that come from a specific config file
func RemoveLogPathsFromConfig(configFile string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	for path, cache := range logCache {
		if cache.ConfigFile == configFile {
			delete(logCache, path)
		}
	}
}

// GetAllLogPaths returns all cached log paths
func GetAllLogPaths(filters ...func(*NginxLogCache) bool) []*NginxLogCache {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	result := make([]*NginxLogCache, 0, len(logCache))
	for _, cache := range logCache {
		flag := true
		if len(filters) > 0 {
			for _, filter := range filters {
				if !filter(cache) {
					flag = false
					break
				}
			}
		}
		if flag {
			result = append(result, cache)
		}
	}

	return result
}

// ClearLogCache clears all entries in the log cache
func ClearLogCache() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Clear the cache
	logCache = make(map[string]*NginxLogCache)
}

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
				logWithIndex.LastModified = &persistenceIndex.LastModified
			}
			logWithIndex.LastSize = persistenceIndex.LastSize
			if !persistenceIndex.LastIndexed.IsZero() {
				logWithIndex.LastIndexed = &persistenceIndex.LastIndexed
			}
			if persistenceIndex.TimeRangeStart != nil {
				logWithIndex.TimeRangeStart = persistenceIndex.TimeRangeStart
				logWithIndex.HasTimeRange = true
			}
			if persistenceIndex.TimeRangeEnd != nil {
				logWithIndex.TimeRangeEnd = persistenceIndex.TimeRangeEnd
				logWithIndex.HasTimeRange = true
			}
			logWithIndex.DocumentCount = persistenceIndex.DocumentCount
		} else if fileStatus, ok := indexedFiles[cache.Path]; ok {
			// Fallback to old index status system
			if logWithIndex.IndexStatus != IndexStatusIndexing {
				logWithIndex.IndexStatus = IndexStatusIndexed
			}
			if !fileStatus.LastModified.IsZero() {
				logWithIndex.LastModified = &fileStatus.LastModified
			}
			logWithIndex.LastSize = fileStatus.LastSize
			if !fileStatus.LastIndexed.IsZero() {
				logWithIndex.LastIndexed = &fileStatus.LastIndexed
			}
			logWithIndex.IsCompressed = fileStatus.IsCompressed
			logWithIndex.HasTimeRange = fileStatus.HasTimeRange
			if !fileStatus.TimeRangeStart.IsZero() {
				logWithIndex.TimeRangeStart = &fileStatus.TimeRangeStart
			}
			if !fileStatus.TimeRangeEnd.IsZero() {
				logWithIndex.TimeRangeEnd = &fileStatus.TimeRangeEnd
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
