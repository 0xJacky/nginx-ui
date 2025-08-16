package nginx_log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/fsnotify/fsnotify"
	"github.com/uozi-tech/cosy/logger"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

const (
	// MinIndexInterval is the minimum interval between two index operations for the same file
	MinIndexInterval = 30 * time.Second
)

// LogIndexer provides high-performance log indexing and querying capabilities
type LogIndexer struct {
	indexPath  string
	index      bleve.Index
	cache      *ristretto.Cache[string, *CachedSearchResult]
	statsCache *ristretto.Cache[string, *CachedStatsResult]
	parser     *LogParser
	watcher    *fsnotify.Watcher
	logPaths   map[string]*LogFileInfo
	mu         sync.RWMutex

	// Background processing
	ctx          context.Context
	cancel       context.CancelFunc
	indexQueue   chan *IndexTask
	indexingLock sync.Map // map[string]*sync.Mutex for per-file locking

	// File debouncing
	debounceTimers sync.Map // map[string]*time.Timer for per-file debouncing
	lastIndexTime  sync.Map // map[string]time.Time for tracking last index time

	// Persistence
	persistence *PersistenceManager

	// Configuration
	maxCacheSize int64
	indexBatch   int
}

// NewLogIndexer creates a new log indexer instance
func NewLogIndexer() (*LogIndexer, error) {
	// Use nginx-ui config directory for index storage
	configDir := filepath.Dir(cosysettings.ConfPath)
	if configDir == "" {
		return nil, fmt.Errorf("nginx-ui config directory not found")
	}

	indexPath := filepath.Join(configDir, "log-index")

	// Create index directory if it doesn't exist
	if err := os.MkdirAll(indexPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create index directory: %w", err)
	}

	// Create or open Bleve index
	index, err := createOrOpenIndex(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create/open index: %w", err)
	}

	// Initialize cache with 100MB capacity
	cache, err := ristretto.NewCache(&ristretto.Config[string, *CachedSearchResult]{
		NumCounters: 1e7,     // number of keys to track frequency of (10M)
		MaxCost:     1 << 27, // maximum cost of cache (128MB)
		BufferItems: 64,      // number of keys per Get buffer
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Initialize statistics cache with 50MB capacity
	statsCache, err := ristretto.NewCache(&ristretto.Config[string, *CachedStatsResult]{
		NumCounters: 1e5,     // number of keys to track frequency of (100K)
		MaxCost:     1 << 26, // maximum cost of cache (64MB)
		BufferItems: 64,      // number of keys per Get buffer
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create stats cache: %w", err)
	}

	// Initialize file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Create user agent parser
	userAgent := NewSimpleUserAgentParser()
	parser := NewLogParser(userAgent)

	// Create context for background processing
	ctx, cancel := context.WithCancel(context.Background())

	indexer := &LogIndexer{
		indexPath:    indexPath,
		index:        index,
		cache:        cache,
		statsCache:   statsCache,
		parser:       parser,
		watcher:      watcher,
		logPaths:     make(map[string]*LogFileInfo),
		ctx:          ctx,
		cancel:       cancel,
		indexQueue:   make(chan *IndexTask, 100), // Buffer up to 100 tasks
		persistence:  NewPersistenceManager(),
		maxCacheSize: 100 * 1024 * 1024, // 100MB
		indexBatch:   10000,             // Index 10000 entries at a time for better performance
	}

	// Start background workers
	go indexer.watchFiles()
	go indexer.processIndexQueue()

	return indexer, nil
}

// createOrOpenIndex creates a new Bleve index or opens an existing one
func createOrOpenIndex(indexPath string) (bleve.Index, error) {
	// Try to open existing index first
	index, err := bleve.Open(indexPath)
	if err == nil {
		return index, nil
	}

	// Create new index if opening failed
	mapping := createIndexMapping()
	index, err = bleve.New(indexPath, mapping)
	if err != nil {
		return nil, fmt.Errorf("failed to create new index: %w", err)
	}

	return index, nil
}

// createIndexMapping creates the Bleve index mapping for log entries
func createIndexMapping() mapping.IndexMapping {
	// Create a mapping for log entries
	logMapping := bleve.NewDocumentMapping()

	// Field mappings - use JSON tag names to match IndexedLogEntry
	timestampMapping := bleve.NewDateTimeFieldMapping()
	logMapping.AddFieldMappingsAt("timestamp", timestampMapping)

	textMapping := bleve.NewTextFieldMapping()
	textMapping.Store = true
	textMapping.Index = true
	logMapping.AddFieldMappingsAt("ip", textMapping)
	logMapping.AddFieldMappingsAt("location", textMapping)
	logMapping.AddFieldMappingsAt("method", textMapping)
	logMapping.AddFieldMappingsAt("path", textMapping)
	logMapping.AddFieldMappingsAt("protocol", textMapping)
	logMapping.AddFieldMappingsAt("referer", textMapping)
	logMapping.AddFieldMappingsAt("user_agent", textMapping)
	logMapping.AddFieldMappingsAt("browser", textMapping)
	logMapping.AddFieldMappingsAt("browser_version", textMapping)
	logMapping.AddFieldMappingsAt("os", textMapping)
	logMapping.AddFieldMappingsAt("os_version", textMapping)
	logMapping.AddFieldMappingsAt("device_type", textMapping)
	logMapping.AddFieldMappingsAt("raw", textMapping)

	numericMapping := bleve.NewNumericFieldMapping()
	numericMapping.Store = true
	numericMapping.Index = true
	logMapping.AddFieldMappingsAt("status", numericMapping)
	logMapping.AddFieldMappingsAt("bytes_sent", numericMapping)
	logMapping.AddFieldMappingsAt("request_time", numericMapping)

	// Create index mapping
	indexMapping := bleve.NewIndexMapping()
	indexMapping.AddDocumentMapping("log_entry", logMapping)

	return indexMapping
}

// RebuildIndex forces a complete rebuild of the index
func (li *LogIndexer) RebuildIndex() error {
	logger.Infof("Starting index rebuild...")

	// Get all files that need to be marked as indexing
	var allLogPaths []string
	if li.persistence != nil {
		indexes, err := li.persistence.GetAllLogIndexes()
		if err != nil {
			logger.Warnf("Failed to get log indexes: %v", err)
		} else {
			for _, logIndex := range indexes {
				allLogPaths = append(allLogPaths, logIndex.Path)
			}
		}
	}

	// Mark all files as being indexed
	for _, path := range allLogPaths {
		SetIndexingStatus(path, true)
	}

	// Update global indexing status
	statusManager := GetIndexingStatusManager()
	statusManager.UpdateIndexingStatus()

	// Close current index
	if err := li.index.Close(); err != nil {
		logger.Warnf("Failed to close index: %v", err)
	}

	// Remove index directory
	if err := os.RemoveAll(li.indexPath); err != nil {
		// Clear indexing status on error
		for _, path := range allLogPaths {
			SetIndexingStatus(path, false)
		}
		statusManager := GetIndexingStatusManager()
		statusManager.UpdateIndexingStatus()
		return fmt.Errorf("failed to remove index directory: %w", err)
	}

	// Create new index
	mapping := createIndexMapping()
	index, err := bleve.New(li.indexPath, mapping)
	if err != nil {
		// Clear indexing status on error
		for _, path := range allLogPaths {
			SetIndexingStatus(path, false)
		}
		statusManager := GetIndexingStatusManager()
		statusManager.UpdateIndexingStatus()
		return fmt.Errorf("failed to create new index: %w", err)
	}
	li.index = index

	// Reset file tracking
	li.mu.Lock()
	for path := range li.logPaths {
		li.logPaths[path].LastModified = time.Time{}
		li.logPaths[path].LastSize = 0
		li.logPaths[path].LastIndexed = time.Time{}
	}
	li.mu.Unlock()

	// Reset persistence data - clear all index position records
	if li.persistence != nil {
		// Get all log indexes
		indexes, err := li.persistence.GetAllLogIndexes()
		if err != nil {
			logger.Warnf("Failed to get log indexes for reset: %v", err)
		} else {
			// Reset each index record
			for _, logIndex := range indexes {
				logIndex.Reset() // Clear position data
				if err := li.persistence.SaveLogIndex(logIndex); err != nil {
					logger.Warnf("Failed to reset log index for %s: %v", logIndex.Path, err)
				}
			}
			logger.Infof("Reset %d persistence records", len(indexes))
		}
	}

	// Clear caches since all data will be reindexed
	li.cache.Clear()
	li.statsCache.Clear()

	// Force re-discovery and re-indexing of all log files
	go func() {
		logger.Infof("Starting background re-indexing of all discovered files")

		// Re-discover and index all log files from nginx configuration
		accessLogPath := nginx.GetAccessLogPath()
		if accessLogPath != "" && IsLogPathUnderWhiteList(accessLogPath) {
			logDir := filepath.Dir(accessLogPath)
			baseLogName := filepath.Base(accessLogPath)

			if err := li.DiscoverLogFiles(logDir, baseLogName); err != nil {
				logger.Errorf("Failed to re-discover log files: %v", err)
			} else {
				logger.Infof("Re-discovery completed, indexing will proceed in background")
			}
		}

		// Also re-index any files that were already tracked
		li.mu.RLock()
		var trackedPaths []string
		for path := range li.logPaths {
			trackedPaths = append(trackedPaths, path)
		}
		li.mu.RUnlock()

		for _, path := range trackedPaths {
			if err := li.ForceReindexFile(path); err != nil {
				logger.Warnf("Failed to force reindex file %s: %v", path, err)
			}
		}

		logger.Infof("Background re-indexing initiated for %d tracked files", len(trackedPaths))
	}()

	logger.Infof("Index rebuild completed, background re-indexing started")
	return nil
}

// GetTimeRange returns the time range of indexed logs
func (li *LogIndexer) GetTimeRange() (start, end time.Time) {
	logger.Infof("GetTimeRange called")

	// First try from memory cache
	li.mu.RLock()
	memoryPathCount := len(li.logPaths)
	for path, fileInfo := range li.logPaths {
		if fileInfo.TimeRange != nil && 
		   !fileInfo.TimeRange.Start.IsZero() && !fileInfo.TimeRange.End.IsZero() {
			if start.IsZero() || fileInfo.TimeRange.Start.Before(start) {
				start = fileInfo.TimeRange.Start
			}
			if end.IsZero() || fileInfo.TimeRange.End.After(end) {
				end = fileInfo.TimeRange.End
			}
			logger.Infof("Memory: valid TimeRange for %s: %v to %v", path, fileInfo.TimeRange.Start, fileInfo.TimeRange.End)
		}
	}
	li.mu.RUnlock()

	// If memory cache didn't provide results, check persistence
	if start.IsZero() || end.IsZero() {
		logger.Infof("Memory cache incomplete (%d paths), checking persistence", memoryPathCount)
		indexes, err := li.persistence.GetAllLogIndexes()
		if err != nil {
			logger.Warnf("Failed to get persistence indexes: %v", err)
		} else {
			for _, logIndex := range indexes {
				if logIndex.TimeRangeStart != nil && logIndex.TimeRangeEnd != nil &&
				   !logIndex.TimeRangeStart.IsZero() && !logIndex.TimeRangeEnd.IsZero() {
					if start.IsZero() || logIndex.TimeRangeStart.Before(start) {
						start = *logIndex.TimeRangeStart
					}
					if end.IsZero() || logIndex.TimeRangeEnd.After(end) {
						end = *logIndex.TimeRangeEnd
					}
					logger.Infof("Persistence: valid TimeRange for %s: %v to %v", logIndex.Path, *logIndex.TimeRangeStart, *logIndex.TimeRangeEnd)
				}
			}
		}
	}

	logger.Infof("GetTimeRange result - start: %v, end: %v", start, end)
	return start, end
}

// GetTimeRangeForPath returns the time range for a specific log file
func (li *LogIndexer) GetTimeRangeForPath(logPath string) (start, end time.Time) {
	logger.Infof("GetTimeRangeForPath called for path: %s", logPath)

	// First try to get from memory cache
	li.mu.RLock()
	fileInfo, exists := li.logPaths[logPath]
	li.mu.RUnlock()

	if exists && fileInfo.TimeRange != nil && 
	   !fileInfo.TimeRange.Start.IsZero() && !fileInfo.TimeRange.End.IsZero() {
		logger.Infof("GetTimeRangeForPath result from memory for %s - start: %v, end: %v",
			logPath, fileInfo.TimeRange.Start, fileInfo.TimeRange.End)
		return fileInfo.TimeRange.Start, fileInfo.TimeRange.End
	}

	// Fallback to persistence data
	logger.Infof("Memory cache miss, checking persistence for %s", logPath)
	logIndex, err := li.persistence.GetLogIndex(logPath)
	if err != nil {
		logger.Warnf("Failed to get persistence data for %s: %v", logPath, err)
		return time.Time{}, time.Time{}
	}

	if logIndex.TimeRangeStart != nil && logIndex.TimeRangeEnd != nil &&
	   !logIndex.TimeRangeStart.IsZero() && !logIndex.TimeRangeEnd.IsZero() {
		logger.Infof("GetTimeRangeForPath result from persistence for %s - start: %v, end: %v",
			logPath, *logIndex.TimeRangeStart, *logIndex.TimeRangeEnd)
		return *logIndex.TimeRangeStart, *logIndex.TimeRangeEnd
	}

	logger.Warnf("No valid time range found for %s", logPath)
	return time.Time{}, time.Time{}
}

// GetIndexStatus returns comprehensive status and statistics about the indexer
func (li *LogIndexer) GetIndexStatus() (*IndexStatus, error) {
	li.mu.RLock()
	defer li.mu.RUnlock()

	// Get document count
	docCount, err := li.index.DocCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get document count: %w", err)
	}

	// Build log paths list
	logPaths := make([]string, 0, len(li.logPaths))
	for path := range li.logPaths {
		logPaths = append(logPaths, path)
	}

	// Build detailed file status list
	files := make([]FileStatus, 0, len(li.logPaths))
	for path, fileInfo := range li.logPaths {
		fileStatus := FileStatus{
			Path:         path,
			LastModified: fileInfo.LastModified,
			LastSize:     fileInfo.LastSize,
			LastIndexed:  fileInfo.LastIndexed,
			IsCompressed: fileInfo.IsCompressed,
			HasTimeRange: fileInfo.TimeRange != nil,
		}

		if fileInfo.TimeRange != nil {
			fileStatus.TimeRangeStart = fileInfo.TimeRange.Start
			fileStatus.TimeRangeEnd = fileInfo.TimeRange.End
		}

		files = append(files, fileStatus)
	}

	// Build status struct
	status := &IndexStatus{
		DocumentCount: docCount,
		LogPaths:      logPaths,
		LogPathsCount: len(logPaths),
		TotalFiles:    len(li.logPaths),
		Files:         files,
	}

	return status, nil
}

// Close closes the indexer and releases resources
func (li *LogIndexer) Close() error {
	// Cancel background processing
	if li.cancel != nil {
		li.cancel()
	}

	// Close file watcher
	if li.watcher != nil {
		li.watcher.Close()
	}

	// Cancel all debounce timers
	li.debounceTimers.Range(func(key, value interface{}) bool {
		if timer, ok := value.(*time.Timer); ok {
			timer.Stop()
		}
		li.debounceTimers.Delete(key)
		return true
	})

	// Close cache
	if li.cache != nil {
		li.cache.Close()
	}

	// Close stats cache
	if li.statsCache != nil {
		li.statsCache.Close()
	}

	// Close index
	if li.index != nil {
		return li.index.Close()
	}

	return nil
}

// DeleteFileIndex removes all indexed data for a specific log file
func (li *LogIndexer) DeleteFileIndex(filePath string) error {
	logger.Infof("Deleting index data for file: %s", filePath)

	// Delete from Bleve index
	query := bleve.NewTermQuery(filePath)
	query.SetField("file_path")
	searchReq := bleve.NewSearchRequest(query)
	searchReq.Size = 10000 // Process in batches

	totalDeleted := 0
	for {
		searchResult, err := li.index.Search(searchReq)
		if err != nil {
			return fmt.Errorf("failed to search existing entries: %w", err)
		}

		if len(searchResult.Hits) == 0 {
			break
		}

		// Delete existing entries
		batch := li.index.NewBatch()
		for _, hit := range searchResult.Hits {
			batch.Delete(hit.ID)
		}

		if err := li.index.Batch(batch); err != nil {
			return fmt.Errorf("failed to delete existing entries: %w", err)
		}

		totalDeleted += len(searchResult.Hits)
		logger.Infof("Deleted %d entries from %s", len(searchResult.Hits), filePath)
	}

	// Remove from in-memory tracking
	li.mu.Lock()
	delete(li.logPaths, filePath)
	li.mu.Unlock()

	// Remove from persistence
	if err := li.persistence.DeleteLogIndex(filePath); err != nil {
		logger.Warnf("Failed to delete log index record: %v", err)
	}

	// Remove from file watcher
	if li.watcher != nil {
		li.watcher.Remove(filePath)
	}

	// Invalidate caches
	li.cache.Clear()
	li.statsCache.Clear()

	logger.Infof("Successfully deleted index for file %s (total entries: %d)", filePath, totalDeleted)
	return nil
}

// DeleteAllIndexes removes all indexed data
func (li *LogIndexer) DeleteAllIndexes() error {
	logger.Infof("Deleting all index data")

	// Close current index
	if err := li.index.Close(); err != nil {
		logger.Warnf("Failed to close index: %v", err)
	}

	// Remove index directory
	if err := os.RemoveAll(li.indexPath); err != nil {
		return fmt.Errorf("failed to remove index directory: %w", err)
	}

	// Create new empty index
	mapping := createIndexMapping()
	index, err := bleve.New(li.indexPath, mapping)
	if err != nil {
		return fmt.Errorf("failed to create new index: %w", err)
	}
	li.index = index

	// Clear in-memory tracking
	li.mu.Lock()
	li.logPaths = make(map[string]*LogFileInfo)
	li.mu.Unlock()

	// Clear all persistence records
	if db := model.UseDB(); db != nil {
		if err := db.Where("1 = 1").Delete(&model.NginxLogIndex{}).Error; err != nil {
			logger.Warnf("Failed to clear persistence records: %v", err)
		}
	}

	// Clear caches
	li.cache.Clear()
	li.statsCache.Clear()

	logger.Infof("Successfully deleted all index data")
	return nil
}

// CleanupOrphanedIndexes removes index data for files that no longer exist
func (li *LogIndexer) CleanupOrphanedIndexes() error {
	logger.Infof("Cleaning up orphaned index data")

	// Get all persistence records
	indexes, err := li.persistence.GetAllLogIndexes()
	if err != nil {
		return fmt.Errorf("failed to get log indexes: %w", err)
	}

	var orphanedPaths []string
	for _, logIndex := range indexes {
		// Check if file still exists
		if _, err := os.Stat(logIndex.Path); os.IsNotExist(err) {
			orphanedPaths = append(orphanedPaths, logIndex.Path)
		}
	}

	// Delete orphaned indexes
	for _, path := range orphanedPaths {
		if err := li.DeleteFileIndex(path); err != nil {
			logger.Warnf("Failed to delete orphaned index for %s: %v", path, err)
		} else {
			logger.Infof("Cleaned up orphaned index for: %s", path)
		}
	}

	logger.Infof("Cleanup completed, removed %d orphaned indexes", len(orphanedPaths))
	return nil
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

// debounceIndexTask implements file-level debouncing for index operations
func (li *LogIndexer) debounceIndexTask(task *IndexTask) {
	filePath := task.FilePath

	// Check if we need to respect the minimum interval
	if lastTime, exists := li.lastIndexTime.Load(filePath); exists {
		if lastIndexTime, ok := lastTime.(time.Time); ok {
			timeSinceLastIndex := time.Since(lastIndexTime)
			if timeSinceLastIndex < MinIndexInterval {
				// Calculate remaining wait time
				remainingWait := MinIndexInterval - timeSinceLastIndex

				// Cancel any existing timer for this file
				if timerInterface, exists := li.debounceTimers.Load(filePath); exists {
					if timer, ok := timerInterface.(*time.Timer); ok {
						timer.Stop()
					}
				}

				// Set new timer
				timer := time.AfterFunc(remainingWait, func() {
					// Clean up timer
					li.debounceTimers.Delete(filePath)
					// Execute the actual indexing
					li.executeIndexTask(task)
				})

				li.debounceTimers.Store(filePath, timer)
				return
			}
		}
	}

	// No debouncing needed, execute immediately
	li.executeIndexTask(task)
}

// executeIndexTask executes the actual indexing task and updates last index time
func (li *LogIndexer) executeIndexTask(task *IndexTask) {
	// Update last index time before processing
	li.lastIndexTime.Store(task.FilePath, time.Now())

	// Queue the task for processing
	select {
	case li.indexQueue <- task:
		// Task queued successfully (no debug log to avoid spam)
	default:
		logger.Warnf("Index queue is full, dropping task for file: %s", task.FilePath)
	}
}
