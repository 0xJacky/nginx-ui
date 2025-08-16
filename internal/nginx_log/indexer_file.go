package nginx_log

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/blevesearch/bleve/v2"
	"github.com/fsnotify/fsnotify"
	"github.com/uozi-tech/cosy/logger"
)

// safeGetFileInfo safely gets file information after validating the path
func (li *LogIndexer) safeGetFileInfo(filePath string) (os.FileInfo, error) {
	// Validate path is under whitelist before accessing
	if !IsLogPathUnderWhiteList(filePath) {
		return nil, fmt.Errorf("file path not under whitelist: %s", filePath)
	}
	
	// Additional validation using isValidLogPath
	if !isValidLogPath(filePath) {
		return nil, fmt.Errorf("invalid log path: %s", filePath)
	}
	
	return os.Stat(filePath)
}

// safeOpenFile safely opens a file after validating the path
func (li *LogIndexer) safeOpenFile(filePath string) (*os.File, error) {
	// Validate path is under whitelist before accessing
	if !IsLogPathUnderWhiteList(filePath) {
		return nil, fmt.Errorf("file path not under whitelist: %s", filePath)
	}
	
	// Additional validation using isValidLogPath
	if !isValidLogPath(filePath) {
		return nil, fmt.Errorf("invalid log path: %s", filePath)
	}
	
	return os.Open(filePath)
}

// safeReadDir safely reads a directory after validating the path
func (li *LogIndexer) safeReadDir(dirPath string) ([]os.DirEntry, error) {
	// Validate directory path is under whitelist before accessing
	if !IsLogPathUnderWhiteList(dirPath) {
		return nil, fmt.Errorf("directory path not under whitelist: %s", dirPath)
	}
	
	return os.ReadDir(dirPath)
}

// AddLogPath adds a log path to be indexed and monitored
func (li *LogIndexer) AddLogPath(logPath string) error {
	li.mu.Lock()
	defer li.mu.Unlock()

	// Check if file exists using safe method
	info, err := li.safeGetFileInfo(logPath)
	if err != nil {
		return fmt.Errorf("failed to safely stat log file %s: %w", logPath, err)
	}

	// Determine if file is compressed
	isCompressed := strings.HasSuffix(logPath, ".gz") || strings.HasSuffix(logPath, ".bz2")

	li.logPaths[logPath] = &LogFileInfo{
		Path:         logPath,
		LastModified: time.Time{}, // Force re-indexing by setting zero time
		LastSize:     0,           // Force re-indexing by setting zero size
		IsCompressed: isCompressed,
	}

	logger.Infof("Added log path %s (actual: mod=%v, size=%d, compressed=%v)",
		logPath, info.ModTime(), info.Size(), isCompressed)

	// Add to file watcher if not compressed and watcher is available
	if li.watcher != nil && !isCompressed {
		if err := li.watcher.Add(logPath); err != nil {
			logger.Warnf("Failed to add file watcher for %s: %v", logPath, err)
		}
	}

	// Also watch the directory for compressed files if watcher is available
	if li.watcher != nil {
		dir := filepath.Dir(logPath)
		if err := li.watcher.Add(dir); err != nil {
			logger.Warnf("Failed to add directory watcher for %s: %v", dir, err)
		}
	}

	// Check if file needs incremental or full indexing
	logIndex, err := li.persistence.GetLogIndex(logPath)
	if err != nil {
		logger.Warnf("Failed to get log index record for %s: %v", logPath, err)
	}

	// Calculate total index size of related log files for comparison
	totalSize := li.calculateRelatedLogFilesSize(logPath)
	needsFullReindex := logIndex == nil || logIndex.ShouldFullReindex(info.ModTime(), totalSize)

	// Queue for background indexing
	li.queueIndexTask(&IndexTask{
		FilePath:    logPath,
		Priority:    1, // Normal priority
		FullReindex: needsFullReindex,
	})

	return nil
}

// processIndexQueue processes indexing tasks in the background
func (li *LogIndexer) processIndexQueue() {
	for {
		select {
		case <-li.ctx.Done():
			logger.Info("Log indexer background processor stopping")
			return
		case task := <-li.indexQueue:
			li.processIndexTask(task)
		}
	}
}

// processIndexTask processes a single indexing task with file locking
func (li *LogIndexer) processIndexTask(task *IndexTask) {
	// Get or create a mutex for this file
	mutexInterface, _ := li.indexingLock.LoadOrStore(task.FilePath, &sync.Mutex{})
	fileMutex := mutexInterface.(*sync.Mutex)

	// Lock the file for indexing
	fileMutex.Lock()
	defer fileMutex.Unlock()

	logger.Infof("Processing index task for file: %s (priority: %d, full_reindex: %v)", task.FilePath, task.Priority, task.FullReindex)

	// Create a context with timeout for this task
	ctx, cancel := context.WithTimeout(li.ctx, 10*time.Minute)
	defer cancel()

	// Check if context is still valid
	select {
	case <-ctx.Done():
		logger.Warnf("Index task cancelled for file: %s", task.FilePath)
		return
	default:
	}

	// Perform the actual indexing
	if err := li.IndexLogFileWithMode(task.FilePath, task.FullReindex); err != nil {
		logger.Errorf("Failed to index file %s: %v", task.FilePath, err)
	} else {
		logger.Infof("Successfully indexed file: %s", task.FilePath)
		// Send index ready notification after successful indexing
		li.notifyIndexReady(task.FilePath)
	}
}

// queueIndexTask adds a task to the indexing queue with debouncing
func (li *LogIndexer) queueIndexTask(task *IndexTask) {
	// Apply debouncing for file updates (not for manual rebuilds)
	if task.Priority < 10 { // Priority 10 is for manual rebuilds, should not be debounced
		li.debounceIndexTask(task)
	} else {
		// Manual rebuilds bypass debouncing
		li.executeIndexTask(task)
	}
}

// IndexLogFileWithMode indexes a log file with specified mode (full or incremental)
func (li *LogIndexer) IndexLogFileWithMode(filePath string, fullReindex bool) error {
	if fullReindex {
		return li.IndexLogFileFull(filePath)
	}
	return li.IndexLogFileIncremental(filePath)
}

// IndexLogFile indexes a specific log file (backward compatibility)
func (li *LogIndexer) IndexLogFile(filePath string) error {
	return li.IndexLogFileWithMode(filePath, false)
}

// IndexLogFileFull performs full reindexing of a log file
func (li *LogIndexer) IndexLogFileFull(filePath string) error {
	logger.Infof("Starting full reindex of log file: %s", filePath)

	// Get status manager and notify indexing started
	statusManager := GetIndexingStatusManager()
	statusManager.NotifyFileIndexingStarted(filePath)
	defer statusManager.NotifyFileIndexingCompleted(filePath)

	// Get or create log index record
	logIndex, err := li.persistence.GetLogIndex(filePath)
	if err != nil {
		return fmt.Errorf("failed to get log index record: %w", err)
	}

	// Get current file info using safe method
	currentInfo, err := li.safeGetFileInfo(filePath)
	if err != nil {
		return fmt.Errorf("failed to safely stat file %s: %w", filePath, err)
	}

	logger.Infof("Full reindexing log file: %s (size: %d, mod: %v)", filePath, currentInfo.Size(), currentInfo.ModTime())

	// Reset log index position for full reindex
	logIndex.Reset()

	// Delete existing entries from this file in the index
	query := bleve.NewTermQuery(filePath)
	query.SetField("file_path")
	searchReq := bleve.NewSearchRequest(query)
	searchReq.Size = 10000 // Process in batches

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

		logger.Infof("Deleted %d existing entries from %s", len(searchResult.Hits), filePath)
	}

	// Read and index the entire file
	return li.indexFileFromPosition(filePath, 0, logIndex)
}

// IndexLogFileIncremental performs incremental indexing of a log file
func (li *LogIndexer) IndexLogFileIncremental(filePath string) error {
	logger.Infof("Starting incremental index of log file: %s", filePath)

	// Get status manager and notify indexing started
	statusManager := GetIndexingStatusManager()
	statusManager.NotifyFileIndexingStarted(filePath)
	defer statusManager.NotifyFileIndexingCompleted(filePath)

	// Get log index record
	logIndex, err := li.persistence.GetLogIndex(filePath)
	if err != nil {
		return fmt.Errorf("failed to get log index record: %w", err)
	}

	// Get current file info using safe method
	currentInfo, err := li.safeGetFileInfo(filePath)
	if err != nil {
		return fmt.Errorf("failed to safely stat file %s: %w", filePath, err)
	}

	// Calculate total index size of related log files for comparison
	totalSize := li.calculateRelatedLogFilesSize(filePath)
	
	// Check if file needs indexing
	if !logIndex.NeedsIndexing(currentInfo.ModTime(), totalSize) {
		logger.Infof("Skipping %s - file group hasn't changed since last index", filePath)
		return nil
	}

	// Check if we need full reindex instead
	if logIndex.ShouldFullReindex(currentInfo.ModTime(), totalSize) {
		logger.Infof("File %s needs full reindex instead of incremental", filePath)
		return li.IndexLogFileFull(filePath)
	}

	logger.Infof("Incremental indexing log file: %s from position %d", filePath, logIndex.LastPosition)

	// Index from last position
	return li.indexFileFromPosition(filePath, logIndex.LastPosition, logIndex)
}

// indexFileFromPosition indexes a file starting from a specific byte position
func (li *LogIndexer) indexFileFromPosition(filePath string, startPosition int64, logIndex *model.NginxLogIndex) error {
	start := time.Now()

	file, err := li.safeOpenFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to safely open file %s: %w", filePath, err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Seek to start position
	if startPosition > 0 {
		_, err = file.Seek(startPosition, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek to position %d: %w", startPosition, err)
		}
	}

	var reader io.Reader = file

	// Handle compressed files (note: incremental indexing may not work well with compressed files)
	if strings.HasSuffix(filePath, ".gz") {
		if startPosition > 0 {
			return fmt.Errorf("incremental indexing not supported for compressed files")
		}
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	scanner := bufio.NewScanner(reader)
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	// Use streaming processing to avoid loading all lines into memory
	return li.indexFileFromPositionStreaming(filePath, startPosition, logIndex, fileInfo, scanner, start)
}

// indexFileFromPositionStreaming processes file content using streaming approach
func (li *LogIndexer) indexFileFromPositionStreaming(filePath string, startPosition int64, logIndex *model.NginxLogIndex, fileInfo os.FileInfo, scanner *bufio.Scanner, startTime time.Time) error {
	// Record index start time
	logIndex.SetIndexStartTime(startTime)
	var currentPosition int64 = startPosition
	lineCount := 0
	entryCount := 0
	batch := li.index.NewBatch()
	var newTimeStart, newTimeEnd *time.Time

	// Line buffer for batch processing
	const batchLines = 10000 // Process 10000 lines at a time for better performance
	var lineBuffer []string

	// If starting from middle of file, skip partial line
	if startPosition > 0 {
		if scanner.Scan() {
			// Skip the first (potentially partial) line
			line := scanner.Text()
			currentPosition += int64(len(line)) + 1 // +1 for newline
		}
	}

	// Process lines in batches
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lineBuffer = append(lineBuffer, line)
			lineCount++
			currentPosition += int64(len(scanner.Text())) + 1 // +1 for newline

			// Process batch when buffer is full
			if len(lineBuffer) >= batchLines {
				if err := li.processBatchStreaming(lineBuffer, filePath, startPosition, &batch, &entryCount, &newTimeStart, &newTimeEnd); err != nil {
					return err
				}
				// Clear buffer
				lineBuffer = lineBuffer[:0]
			}

			// Progress logging for large files
			if lineCount%10000 == 0 {
				logger.Debugf("Processed %d lines, indexed %d entries from %s...", lineCount, entryCount, filePath)
			}
		}
	}

	// Process remaining lines in buffer
	if len(lineBuffer) > 0 {
		if err := li.processBatchStreaming(lineBuffer, filePath, startPosition, &batch, &entryCount, &newTimeStart, &newTimeEnd); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan file %s: %w", filePath, err)
	}

	// Execute final batch
	if batch.Size() > 0 {
		if err := li.index.Batch(batch); err != nil {
			return fmt.Errorf("failed to execute final batch: %w", err)
		}
	}

	logger.Debugf("Processed %d lines, indexed %d entries from %s (position: %d->%d)", lineCount, entryCount, filePath, startPosition, currentPosition)

	if entryCount == 0 {
		logger.Debugf("No new entries to index in %s", filePath)
		// Still update the position and modification time with total index size
		totalSize := li.calculateRelatedLogFilesSize(filePath)
		// Record index completion time and duration
		logIndex.SetIndexDuration(startTime)
		logIndex.UpdateProgress(fileInfo.ModTime(), totalSize, fileInfo.Size(), logIndex.DocumentCount, logIndex.TimeRangeStart, logIndex.TimeRangeEnd)
		return li.persistence.SaveLogIndex(logIndex)
	}

	// Update time range in log index
	var timeRangeStart, timeRangeEnd *time.Time
	if logIndex.TimeRangeStart != nil {
		timeRangeStart = logIndex.TimeRangeStart
	} else {
		timeRangeStart = newTimeStart
	}
	if logIndex.TimeRangeEnd != nil {
		timeRangeEnd = logIndex.TimeRangeEnd
	} else {
		timeRangeEnd = newTimeEnd
	}

	// Expand time range if needed
	if newTimeStart != nil && (timeRangeStart == nil || newTimeStart.Before(*timeRangeStart)) {
		timeRangeStart = newTimeStart
	}
	if newTimeEnd != nil && (timeRangeEnd == nil || newTimeEnd.After(*timeRangeEnd)) {
		timeRangeEnd = newTimeEnd
	}

	// Calculate total index size of related log files for this log group
	totalSize := li.calculateRelatedLogFilesSize(filePath)
	
	// Record index completion time and duration
	logIndex.SetIndexDuration(startTime)
	
	// Update persistence record with total index size
	logIndex.UpdateProgress(fileInfo.ModTime(), totalSize, currentPosition, logIndex.DocumentCount+uint64(entryCount), timeRangeStart, timeRangeEnd)
	if err := li.persistence.SaveLogIndex(logIndex); err != nil {
		logger.Warnf("Failed to save log index: %v", err)
	}

	// Update in-memory file info for compatibility
	li.mu.Lock()
	if fileInfo, exists := li.logPaths[filePath]; exists {
		fileInfo.LastModified = logIndex.LastModified
		fileInfo.LastSize = logIndex.LastSize
		fileInfo.LastIndexed = logIndex.LastIndexed
		if timeRangeStart != nil && timeRangeEnd != nil {
			fileInfo.TimeRange = &TimeRange{Start: *timeRangeStart, End: *timeRangeEnd}
		}
	}
	li.mu.Unlock()

	// Invalidate statistics cache since data has changed
	li.invalidateStatsCache()

	// Clear indexing status for this file
	SetIndexingStatus(filePath, false)
	statusManager := GetIndexingStatusManager()
	statusManager.UpdateIndexingStatus()

	indexDuration := time.Since(startTime)
	logger.Infof("Indexed %d new entries from %s in %v (position: %d->%d, index_size: %d bytes)",
		entryCount, filePath, indexDuration, startPosition, currentPosition, totalSize)

	// Send index ready notification after successful indexing
	li.notifyIndexReady(filePath)

	return nil
}

// processBatchStreaming processes a batch of lines using parallel parsing
func (li *LogIndexer) processBatchStreaming(lines []string, filePath string, startPosition int64, batch **bleve.Batch, entryCount *int, newTimeStart, newTimeEnd **time.Time) error {
	if len(lines) == 0 {
		return nil
	}

	// Parse lines in parallel
	entries := li.parser.ParseLines(lines)

	if len(entries) == 0 {
		return nil // No valid entries in this batch
	}

	// Index entries
	for i, entry := range entries {
		// Track time range for new entries
		if *newTimeStart == nil || entry.Timestamp.Before(**newTimeStart) {
			*newTimeStart = &entry.Timestamp
		}
		if *newTimeEnd == nil || entry.Timestamp.After(**newTimeEnd) {
			*newTimeEnd = &entry.Timestamp
		}

		// Create indexed entry with unique ID
		indexedEntry := &IndexedLogEntry{
			ID:           fmt.Sprintf("%s_%d_%d", filepath.Base(filePath), startPosition, *entryCount+i),
			FilePath:     filePath,
			Timestamp:    entry.Timestamp,
			IP:           entry.IP,
			Location:     entry.Location,
			Method:       entry.Method,
			Path:         entry.Path,
			Protocol:     entry.Protocol,
			Status:       entry.Status,
			BytesSent:    entry.BytesSent,
			Referer:      entry.Referer,
			UserAgent:    entry.UserAgent,
			Browser:      entry.Browser,
			BrowserVer:   entry.BrowserVer,
			OS:           entry.OS,
			OSVersion:    entry.OSVersion,
			DeviceType:   entry.DeviceType,
			RequestTime:  entry.RequestTime,
			UpstreamTime: entry.UpstreamTime,
			Raw:          entry.Raw,
		}

		(*batch).Index(indexedEntry.ID, indexedEntry)

		// Execute batch when it reaches the limit
		if (*batch).Size() >= li.indexBatch {
			if err := li.index.Batch(*batch); err != nil {
				return fmt.Errorf("failed to execute batch: %w", err)
			}
			*batch = li.index.NewBatch()
		}
	}

	*entryCount += len(entries)
	return nil
}

// isLogrotateFile checks if a filename matches logrotate naming patterns
// Matches: access.log, access.log.1, access.log.2, access.log.10.gz, etc.
// Does NOT match: random.gz, access_20230815.gz, etc.
func isLogrotateFile(filename, baseLogName string) bool {
	// Case 1: Exact match (current log file)
	if filename == baseLogName {
		return true
	}

	// Case 2: Must start with baseLogName followed by a dot
	if !strings.HasPrefix(filename, baseLogName+".") {
		return false
	}

	// Remove the base name and the first dot
	suffix := strings.TrimPrefix(filename, baseLogName+".")

	// Case 3: Rotated file (access.log.1, access.log.2, etc.)
	if matched, _ := regexp.MatchString(`^\d+$`, suffix); matched {
		return true
	}

	// Case 4: Compressed rotated file (access.log.1.gz, access.log.10.gz, etc.)
	if strings.HasSuffix(suffix, ".gz") {
		numberPart := strings.TrimSuffix(suffix, ".gz")
		if matched, _ := regexp.MatchString(`^\d+$`, numberPart); matched {
			return true
		}
	}

	return false
}

// handleCompressedLogFile handles the creation of new compressed log files
func (li *LogIndexer) handleCompressedLogFile(fullPath string) {
	li.mu.RLock()
	defer li.mu.RUnlock()

	fileName := filepath.Base(fullPath)
	for logPath := range li.logPaths {
		baseLogName := filepath.Base(logPath)
		if isLogrotateFile(fileName, baseLogName) {
			go func(path string) {
				if err := li.AddLogPath(path); err != nil {
					logger.Errorf("Failed to add new compressed log file %s: %v", path, err)
					return
				}

				// Queue for full indexing (compressed files need full reindex)
				li.queueIndexTask(&IndexTask{
					FilePath:    path,
					Priority:    1,    // Normal priority for compressed files
					FullReindex: true, // Compressed files need full indexing
				})
			}(fullPath)
			return // Found matching log path, no need to continue
		}
	}
}

// watchFiles watches for file system events
func (li *LogIndexer) watchFiles() {
	for {
		select {
		case <-li.ctx.Done():
			logger.Info("Log indexer file watcher stopping")
			return
		case event, ok := <-li.watcher.Events:
			if !ok {
				return
			}

			// Handle file modifications
			if event.Op&fsnotify.Write == fsnotify.Write {
				li.mu.RLock()
				_, exists := li.logPaths[event.Name]
				li.mu.RUnlock()

				if exists {
					// Queue for incremental indexing (debouncing handled by queueIndexTask)
					li.queueIndexTask(&IndexTask{
						FilePath:    event.Name,
						Priority:    2,     // Higher priority for file updates
						FullReindex: false, // Use incremental indexing for file updates
					})
				}
			}

			// Handle new compressed files
			if event.Op&fsnotify.Create == fsnotify.Create {
				if strings.HasSuffix(event.Name, ".gz") {
					// Check if this is a rotated log file we should index
					li.handleCompressedLogFile(event.Name)
				}
			}

		case err, ok := <-li.watcher.Errors:
			if !ok {
				return
			}
			logger.Errorf("File watcher error: %v", err)
		}
	}
}

// ForceReindexFile forces re-indexing of a specific log file
func (li *LogIndexer) ForceReindexFile(logPath string) error {
	li.mu.Lock()
	fileInfo, exists := li.logPaths[logPath]
	if !exists {
		li.mu.Unlock()
		return fmt.Errorf("log file %s not registered", logPath)
	}

	// Reset file info to force re-indexing
	fileInfo.LastModified = time.Time{}
	fileInfo.LastSize = 0
	fileInfo.TimeRange = nil
	li.mu.Unlock()

	logger.Infof("Force reindexing file: %s", logPath)

	// Mark file as being indexed
	SetIndexingStatus(logPath, true)
	statusManager := GetIndexingStatusManager()
	statusManager.UpdateIndexingStatus()

	// Queue for immediate full reindexing with high priority
	li.queueIndexTask(&IndexTask{
		FilePath:    logPath,
		Priority:    10,   // High priority for manual reindex
		FullReindex: true, // Force full reindex
	})

	return nil
}

// RepairFileMetadata repairs file metadata by scanning existing index data
func (li *LogIndexer) RepairFileMetadata() error {
	logger.Infof("Starting file metadata repair...")

	li.mu.Lock()
	defer li.mu.Unlock()

	for filePath, fileInfo := range li.logPaths {
		logger.Infof("Repairing metadata for: %s", filePath)

		// Check if file exists and get current info
		currentInfo, err := os.Stat(filePath)
		if err != nil {
			logger.Warnf("Failed to stat file %s: %v", filePath, err)
			continue
		}

		// Query index for entries from this file to determine time range
		query := bleve.NewTermQuery(filePath)
		query.SetField("file_path")

		searchReq := bleve.NewSearchRequest(query)
		searchReq.Size = 1000 // Get a sample to determine time range
		searchReq.Fields = []string{"timestamp"}
		searchReq.SortBy([]string{"timestamp"}) // Sort by timestamp

		searchResult, err := li.index.Search(searchReq)
		if err != nil {
			logger.Warnf("Failed to search index for file %s: %v", filePath, err)
			continue
		}

		if searchResult.Total == 0 {
			logger.Warnf("No indexed entries found for file %s", filePath)
			continue
		}

		// Get time range from search results
		var timeRange *TimeRange
		for _, hit := range searchResult.Hits {
			if timestampField, ok := hit.Fields["timestamp"]; ok {
				if timestampStr, ok := timestampField.(string); ok {
					timestamp, err := time.Parse(time.RFC3339, timestampStr)
					if err != nil {
						continue
					}

					if timeRange == nil {
						timeRange = &TimeRange{Start: timestamp, End: timestamp}
					} else {
						if timestamp.Before(timeRange.Start) {
							timeRange.Start = timestamp
						}
						if timestamp.After(timeRange.End) {
							timeRange.End = timestamp
						}
					}
				}
			}
		}

		// Update file info
		fileInfo.LastModified = currentInfo.ModTime()
		fileInfo.LastSize = currentInfo.Size()
		fileInfo.LastIndexed = time.Now()
		fileInfo.TimeRange = timeRange

		if timeRange != nil {
			logger.Infof("Repaired metadata for %s: TimeRange %v to %v, Total entries: %d",
				filePath, timeRange.Start, timeRange.End, searchResult.Total)
		} else {
			logger.Warnf("Could not determine time range for %s", filePath)
		}
	}

	logger.Infof("File metadata repair completed")
	return nil
}

// DiscoverLogFiles discovers log files in a directory, including compressed ones
func (li *LogIndexer) DiscoverLogFiles(logDir string, baseLogName string) error {
	logger.Infof("Auto-discovering log files in %s with base name %s", logDir, baseLogName)

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory %s: %w", logDir, err)
	}

	var logFiles []string

	// Find all log files (current and rotated)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Match logrotate patterns: access.log, access.log.1, access.log.2, access.log.1.gz, etc.
		if isLogrotateFile(name, baseLogName) {
			fullPath := filepath.Join(logDir, name)
			logFiles = append(logFiles, fullPath)
			logger.Debugf("Found matching log file: %s", fullPath)
		}
	}

	if len(logFiles) == 0 {
		logger.Warnf("No log files found matching pattern %s in directory %s", baseLogName, logDir)
		return fmt.Errorf("no log files found matching pattern %s", baseLogName)
	}

	// Sort files to process them in order (newest first for current log)
	sort.Slice(logFiles, func(i, j int) bool {
		// Current log file should be processed first
		if !strings.Contains(logFiles[i], ".") && strings.Contains(logFiles[j], ".") {
			return true
		}
		return logFiles[i] < logFiles[j]
	})

	logger.Infof("Found %d log files to process: %v", len(logFiles), logFiles)

	// Add all discovered log files and queue them for background indexing
	var addedCount int
	for _, logFile := range logFiles {
		logger.Infof("Adding log file: %s", logFile)

		if err := li.AddLogPath(logFile); err != nil {
			logger.Warnf("Failed to add log path %s: %v", logFile, err)
			continue
		}

		addedCount++
		logger.Infof("Successfully added log file %s (queued for indexing)", logFile)
	}

	logger.Infof("Discovered and added %d log files in %s (queued for background indexing)", addedCount, logDir)
	return nil
}

// calculateRelatedLogFilesSize calculates the total index size of all related log files for a given base log
func (li *LogIndexer) calculateRelatedLogFilesSize(baseLogPath string) int64 {
	logDir := filepath.Dir(baseLogPath)
	baseLogName := filepath.Base(baseLogPath)
	
	entries, err := li.safeReadDir(logDir)
	if err != nil {
		logger.Warnf("Failed to read log directory %s: %v", logDir, err)
		// Fallback to single file size using safe method
		if info, err := li.safeGetFileInfo(baseLogPath); err == nil {
			return info.Size()
		}
		return 0
	}
	
	var totalSize int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		name := entry.Name()
		if isLogrotateFile(name, baseLogName) {
			fullPath := filepath.Join(logDir, name)
			// Use safe method to get file info for related log files
			if info, err := li.safeGetFileInfo(fullPath); err == nil {
				totalSize += info.Size()
				logger.Debugf("Added file %s (size: %d) to total size calculation", fullPath, info.Size())
			}
		}
	}
	
	logger.Debugf("Total index size for log group %s: %d bytes", baseLogPath, totalSize)
	return totalSize
}

// notifyIndexReady sends an event notification when index is ready for a specific log path
func (li *LogIndexer) notifyIndexReady(logPath string) {
	// Get time range for the indexed log
	start, end := li.GetTimeRangeForPath(logPath)

	eventData := event.NginxLogIndexReadyData{
		LogPath:     logPath,
		StartTime:   start.Format(time.RFC3339),
		EndTime:     end.Format(time.RFC3339),
		Available:   !start.IsZero() && !end.IsZero(),
		IndexStatus: "ready",
	}

	// Send event notification
	event.Publish(event.Event{
		Type: event.EventTypeNginxLogIndexReady,
		Data: eventData,
	})

	logger.Infof("Sent index ready notification for log path: %s (time range: %v to %v)",
		logPath, start, end)
}
