package indexer

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

// RebuildManager handles index rebuilding operations
type RebuildManager struct {
	indexer         *ParallelIndexer
	persistence     *PersistenceManager
	progressManager *ProgressManager
	shardManager    ShardManager
	config          *RebuildConfig
	rebuilding      int32 // atomic flag
	lastRebuildTime time.Time
	mu              sync.RWMutex
}

// RebuildConfig contains configuration for rebuild operations
type RebuildConfig struct {
	BatchSize           int           `json:"batch_size"`
	MaxConcurrency      int           `json:"max_concurrency"`
	DeleteBeforeRebuild bool          `json:"delete_before_rebuild"`
	ProgressInterval    time.Duration `json:"progress_interval"`
	TimeoutPerFile      time.Duration `json:"timeout_per_file"`
}

// DefaultRebuildConfig returns default rebuild configuration
func DefaultRebuildConfig() *RebuildConfig {
	return &RebuildConfig{
		BatchSize:           1000,
		MaxConcurrency:      4,
		DeleteBeforeRebuild: true,
		ProgressInterval:    5 * time.Second,
		TimeoutPerFile:      30 * time.Minute,
	}
}

// NewRebuildManager creates a new rebuild manager
func NewRebuildManager(indexer *ParallelIndexer, persistence *PersistenceManager, progressManager *ProgressManager, shardManager ShardManager, config *RebuildConfig) *RebuildManager {
	if config == nil {
		config = DefaultRebuildConfig()
	}

	return &RebuildManager{
		indexer:         indexer,
		persistence:     persistence,
		progressManager: progressManager,
		shardManager:    shardManager,
		config:          config,
	}
}

// RebuildAll rebuilds all indexes from scratch
func (rm *RebuildManager) RebuildAll(ctx context.Context) error {
	// Check if already rebuilding
	if !atomic.CompareAndSwapInt32(&rm.rebuilding, 0, 1) {
		return fmt.Errorf("rebuild already in progress")
	}
	defer atomic.StoreInt32(&rm.rebuilding, 0)

	startTime := time.Now()
	rm.mu.Lock()
	rm.lastRebuildTime = startTime
	rm.mu.Unlock()

	// Get all log groups to rebuild
	logGroups, err := rm.getAllLogGroups()
	if err != nil {
		return fmt.Errorf("failed to get log groups: %w", err)
	}

	if len(logGroups) == 0 {
		return fmt.Errorf("no log groups found to rebuild")
	}

	// Delete existing indexes if configured
	if rm.config.DeleteBeforeRebuild {
		if err := rm.deleteAllIndexes(); err != nil {
			return fmt.Errorf("failed to delete existing indexes: %w", err)
		}
	}

	// Reset persistence records
	if rm.persistence != nil {
		if err := rm.resetAllPersistenceRecords(); err != nil {
			return fmt.Errorf("failed to reset persistence records: %w", err)
		}
	}

	// Create progress tracker for overall rebuild
	rebuildProgress := &RebuildProgress{
		TotalGroups:     len(logGroups),
		CompletedGroups: 0,
		StartTime:       startTime,
	}

	// Process each log group
	errors := make([]error, 0)
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, rm.config.MaxConcurrency)

	for _, logGroup := range logGroups {
		wg.Add(1)
		go func(group string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Check context
			if ctx.Err() != nil {
				return
			}

			// Rebuild this log group
			if err := rm.rebuildLogGroup(ctx, group); err != nil {
				rm.mu.Lock()
				errors = append(errors, fmt.Errorf("failed to rebuild group %s: %w", group, err))
				rm.mu.Unlock()
			} else {
				// Update progress
				rm.mu.Lock()
				rebuildProgress.CompletedGroups++
				rm.mu.Unlock()

				// Notify progress
				rm.notifyRebuildProgress(rebuildProgress)
			}
		}(logGroup)
	}

	// Wait for all groups to complete
	wg.Wait()

	// Check for errors
	if len(errors) > 0 {
		return fmt.Errorf("rebuild completed with %d errors: %v", len(errors), errors)
	}

	rebuildProgress.CompletedTime = time.Now()
	rebuildProgress.Duration = time.Since(startTime)

	// Notify completion
	rm.notifyRebuildComplete(rebuildProgress)

	return nil
}

// RebuildSingle rebuilds index for a single log group
func (rm *RebuildManager) RebuildSingle(ctx context.Context, logGroupPath string) error {
	// Check if already rebuilding
	if !atomic.CompareAndSwapInt32(&rm.rebuilding, 0, 1) {
		return fmt.Errorf("rebuild already in progress")
	}
	defer atomic.StoreInt32(&rm.rebuilding, 0)

	startTime := time.Now()

	// Delete existing index for this log group if configured
	if rm.config.DeleteBeforeRebuild {
		if err := rm.deleteLogGroupIndex(logGroupPath); err != nil {
			return fmt.Errorf("failed to delete existing index: %w", err)
		}
	}

	// Reset persistence records for this group
	if rm.persistence != nil {
		if err := rm.resetLogGroupPersistence(logGroupPath); err != nil {
			return fmt.Errorf("failed to reset persistence: %w", err)
		}
	}

	// Rebuild the log group
	if err := rm.rebuildLogGroup(ctx, logGroupPath); err != nil {
		return fmt.Errorf("failed to rebuild log group: %w", err)
	}

	duration := time.Since(startTime)

	// Notify completion
	rm.notifySingleRebuildComplete(logGroupPath, duration)

	return nil
}

// rebuildLogGroup rebuilds index for a single log group
func (rm *RebuildManager) rebuildLogGroup(ctx context.Context, logGroupPath string) error {
	// Get all files for this log group
	files, err := rm.discoverLogGroupFiles(logGroupPath)
	if err != nil {
		return fmt.Errorf("failed to discover files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found for log group %s", logGroupPath)
	}

	// Create progress tracker for this log group
	progressConfig := &ProgressConfig{
		OnProgress: func(pn ProgressNotification) {
			// Handle progress notifications
			rm.handleProgressNotification(logGroupPath, pn)
		},
		OnCompletion: func(cn CompletionNotification) {
			// Handle completion notifications
			rm.handleCompletionNotification(logGroupPath, cn)
		},
	}

	tracker := rm.progressManager.GetTracker(logGroupPath, progressConfig)

	// Add all files to tracker
	for _, file := range files {
		tracker.AddFile(file.Path, file.IsCompressed)
		if file.EstimatedLines > 0 {
			tracker.SetFileEstimate(file.Path, file.EstimatedLines)
		}
		if file.Size > 0 {
			tracker.SetFileSize(file.Path, file.Size)
		}
	}

	// Process files in parallel with controlled concurrency
	var fileWg sync.WaitGroup
	fileSemaphore := make(chan struct{}, rm.config.MaxConcurrency)
	var fileErrors []error
	var fileErrMu sync.Mutex

	for _, file := range files {
		// Check context before starting new file
		if ctx.Err() != nil {
			tracker.FailFile(file.Path, ctx.Err().Error())
			break
		}

		// Skip unchanged files (especially compressed archives)
		shouldProcess, skipReason := rm.shouldProcessFile(file)
		if !shouldProcess {
			logger.Infof("Skipping file %s: %s", file.Path, skipReason)
			// Mark as completed without processing
			tracker.CompleteFile(file.Path, 0)
			continue
		}

		fileWg.Add(1)
		go func(f *LogGroupFile) {
			defer fileWg.Done()

			// Acquire semaphore for controlled concurrency
			fileSemaphore <- struct{}{}
			defer func() { <-fileSemaphore }()

			// Check context again inside goroutine
			if ctx.Err() != nil {
				tracker.FailFile(f.Path, ctx.Err().Error())
				return
			}

			// Create file-specific context with timeout
			fileCtx, cancel := context.WithTimeout(ctx, rm.config.TimeoutPerFile)
			defer cancel()

			// Start processing
			tracker.StartFile(f.Path)

			// Index the file
			err := rm.indexFile(fileCtx, f, tracker)

			if err != nil {
				tracker.FailFile(f.Path, err.Error())
				fileErrMu.Lock()
				fileErrors = append(fileErrors, fmt.Errorf("failed to index file %s: %w", f.Path, err))
				fileErrMu.Unlock()
				return
			}

			// Mark as completed
			tracker.CompleteFile(f.Path, f.ProcessedLines)

			// Update persistence with exact doc count from Bleve
			if rm.persistence != nil {
				exactCount := f.DocumentCount
				if rm.indexer != nil && rm.indexer.IsHealthy() {
					if c, err := rm.indexer.CountDocsByFilePath(f.Path); err == nil {
						exactCount = c
					} else {
						logger.Warnf("Falling back to computed count for %s due to count error: %v", f.Path, err)
					}
				}
				if err := rm.persistence.MarkFileAsIndexed(f.Path, exactCount, f.LastPosition); err != nil {
					// Log but don't fail
					// logger.Warnf("Failed to update persistence for %s: %v", f.Path, err)
				}
			}
		}(file)
	}

	// Wait for all files to complete
	fileWg.Wait()

	// Check for file processing errors
	if len(fileErrors) > 0 {
		return fmt.Errorf("failed to index %d files in group %s: %v", len(fileErrors), logGroupPath, fileErrors[0])
	}

	return nil
}

// shouldProcessFile determines if a file needs to be processed based on change detection
func (rm *RebuildManager) shouldProcessFile(file *LogGroupFile) (bool, string) {
	// Get file information
	fileInfo, err := os.Stat(file.Path)
	if err != nil {
		return true, fmt.Sprintf("cannot stat file (will process): %v", err)
	}

	// For compressed files (.gz), check if we've already processed them and they haven't changed
	if file.IsCompressed {
		// Check if we have persistence information for this file
		if rm.persistence != nil {
			if info, err := rm.persistence.GetIncrementalInfo(file.Path); err == nil {
				// Check if file hasn't changed since last indexing
				currentModTime := fileInfo.ModTime().Unix()
				currentSize := fileInfo.Size()

				if info.LastModified == currentModTime &&
					info.LastSize == currentSize &&
					info.LastPosition == currentSize {
					return false, "compressed file already fully indexed and unchanged"
				}
			}
		}
	}

	// For active log files (non-compressed), always process but may resume from checkpoint
	if !file.IsCompressed {
		// Check if file has grown or changed
		if rm.persistence != nil {
			if info, err := rm.persistence.GetIncrementalInfo(file.Path); err == nil {
				currentModTime := fileInfo.ModTime().Unix()
				currentSize := fileInfo.Size()

				// File hasn't changed at all
				if info.LastModified == currentModTime &&
					info.LastSize == currentSize &&
					info.LastPosition == currentSize {
					return false, "active file unchanged since last indexing"
				}

				// File has shrunk (possible log rotation)
				if currentSize < info.LastSize {
					return true, "active file appears to have been rotated (size decreased)"
				}

				// File has grown or been modified
				if currentSize > info.LastSize || currentModTime > info.LastModified {
					return true, "active file has new content"
				}
			}
		}

		// No persistence info available, process the file
		return true, "no previous indexing record found for active file"
	}

	// Default: process compressed files if no persistence info
	return true, "no previous indexing record found for compressed file"
}

// LogGroupFile represents a file in a log group
type LogGroupFile struct {
	Path           string
	Size           int64
	ModTime        int64 // Unix timestamp of file modification time
	IsCompressed   bool
	EstimatedLines int64
	ProcessedLines int64
	DocumentCount  uint64
	LastPosition   int64
}

// discoverLogGroupFiles discovers all files for a log group
func (rm *RebuildManager) discoverLogGroupFiles(logGroupPath string) ([]*LogGroupFile, error) {
	dir := filepath.Dir(logGroupPath)

	// Remove any rotation suffixes to get the base name
	mainPath := getMainLogPathFromFile(logGroupPath)

	files := make([]*LogGroupFile, 0)

	// Walk the directory to find related files
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if this file belongs to the log group
		if getMainLogPathFromFile(path) == mainPath {
			file := &LogGroupFile{
				Path:         path,
				Size:         info.Size(),
				ModTime:      info.ModTime().Unix(),
				IsCompressed: IsCompressedFile(path),
			}

			// Estimate lines
			ctx := context.Background()
			if lines, err := EstimateFileLines(ctx, path, info.Size(), file.IsCompressed); err == nil {
				file.EstimatedLines = lines
			}

			files = append(files, file)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// indexFile indexes a single file with checkpoint/resume support
func (rm *RebuildManager) indexFile(ctx context.Context, file *LogGroupFile, tracker *ProgressTracker) error {
	// Create a batch writer
	batch := NewBatchWriter(rm.indexer, rm.config.BatchSize)
	defer batch.Flush()

	// Get checkpoint information from persistence layer
	var startPosition int64 = 0
	var resuming bool = false

	if rm.persistence != nil {
		if info, err := rm.persistence.GetIncrementalInfo(file.Path); err == nil {
			// Get current file modification time
			fileInfo, err := os.Stat(file.Path)
			if err != nil {
				return fmt.Errorf("failed to stat file %s: %w", file.Path, err)
			}

			currentModTime := fileInfo.ModTime().Unix()
			currentSize := fileInfo.Size()

			// Check if file hasn't changed since last indexing
			if info.LastIndexed > 0 &&
				info.LastModified == currentModTime &&
				info.LastSize == currentSize &&
				info.LastPosition == currentSize {
				// File hasn't changed and was fully indexed
				logger.Infof("Skipping indexing for unchanged file %s (last indexed: %v)",
					file.Path, time.Unix(info.LastIndexed, 0))
				file.ProcessedLines = 0 // No new lines processed
				file.DocumentCount = 0  // No new documents added
				file.LastPosition = currentSize
				return nil
			}

			// Check if we should resume from a previous position
			if info.LastPosition > 0 && info.LastPosition < currentSize {
				// File has grown since last indexing
				startPosition = info.LastPosition
				resuming = true
				logger.Infof("Resuming indexing from position %d for file %s (file size: %d -> %d)",
					startPosition, file.Path, info.LastSize, currentSize)
			} else if currentSize < info.LastSize {
				// File has been truncated or rotated, start from beginning
				startPosition = 0
				logger.Infof("File %s has been truncated/rotated (size: %d -> %d), reindexing from start",
					file.Path, info.LastSize, currentSize)
			} else if info.LastPosition >= currentSize && currentSize > 0 {
				// File size hasn't changed and we've already processed it completely
				if info.LastModified == currentModTime {
					logger.Infof("File %s already fully indexed and unchanged, skipping", file.Path)
					file.ProcessedLines = 0
					file.DocumentCount = 0
					file.LastPosition = currentSize
					return nil
				}
				// File has same size but different modification time, reindex from start
				startPosition = 0
				logger.Infof("File %s has same size but different mod time, reindexing from start", file.Path)
			}
		}
	}

	// Open file with resume support
	reader, err := rm.openFileFromPosition(file.Path, startPosition)
	if err != nil {
		return fmt.Errorf("failed to open file %s from position %d: %w", file.Path, startPosition, err)
	}
	defer reader.Close()

	// Process file line by line with checkpointing
	var processedLines int64 = 0
	var currentPosition int64 = startPosition
	var documentCount uint64 = 0
	checkpointInterval := int64(1000) // Save checkpoint every 1000 lines

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		// Check context for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		currentPosition += int64(len(line)) + 1 // +1 for newline

		// Process the log line (parse and add to batch)
		// This would typically involve:
		// 1. Parse log entry using parser
		// 2. Create search document
		// 3. Add to batch

		processedLines++
		documentCount++

		// Update progress
		tracker.UpdateFileProgress(file.Path, processedLines)

		// Periodic checkpoint saving
		if processedLines%checkpointInterval == 0 {
			if rm.persistence != nil {
				// Get current file modification time for checkpoint
				fileInfo, err := os.Stat(file.Path)
				var modTime int64
				if err == nil {
					modTime = fileInfo.ModTime().Unix()
				} else {
					modTime = time.Now().Unix()
				}

				info := &LogFileInfo{
					Path:         file.Path,
					LastPosition: currentPosition,
					LastIndexed:  time.Now().Unix(),
					LastModified: modTime,
					LastSize:     file.Size,
				}
				if err := rm.persistence.UpdateIncrementalInfo(file.Path, info); err != nil {
					logger.Warnf("Failed to save checkpoint for %s: %v", file.Path, err)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file %s: %w", file.Path, err)
	}

	// Update file statistics
	file.ProcessedLines = processedLines
	file.DocumentCount = documentCount
	file.LastPosition = currentPosition

	// Save final checkpoint
	if rm.persistence != nil {
		// Get current file info for accurate metadata
		fileInfo, err := os.Stat(file.Path)
		var modTime int64
		if err == nil {
			modTime = fileInfo.ModTime().Unix()
		} else {
			modTime = time.Now().Unix()
		}

		info := &LogFileInfo{
			Path:         file.Path,
			LastPosition: currentPosition,
			LastIndexed:  time.Now().Unix(),
			LastModified: modTime,
			LastSize:     file.Size,
		}
		if err := rm.persistence.UpdateIncrementalInfo(file.Path, info); err != nil {
			logger.Warnf("Failed to save final checkpoint for %s: %v", file.Path, err)
		}
	}

	if resuming {
		logger.Infof("Completed resumed indexing for %s: %d lines, %d documents",
			file.Path, processedLines, documentCount)
	}

	return nil
}

// openFileFromPosition opens a file and seeks to the specified position
// Handles both compressed (.gz) and regular files
func (rm *RebuildManager) openFileFromPosition(filePath string, startPosition int64) (io.ReadCloser, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	// Check if file is compressed
	isGzipped := strings.HasSuffix(filePath, ".gz")

	if isGzipped {
		// For gzip files, we need to read from the beginning and skip to position
		// This is because gzip doesn't support random seeking
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}

		if startPosition > 0 {
			// Skip to the start position by reading and discarding bytes
			_, err := io.CopyN(io.Discard, gzReader, startPosition)
			if err != nil && err != io.EOF {
				gzReader.Close()
				file.Close()
				return nil, fmt.Errorf("failed to seek to position %d in gzip file: %w", startPosition, err)
			}
		}

		// Return a wrapped reader that closes both gzReader and file
		return &gzipReaderCloser{gzReader: gzReader, file: file}, nil
	} else {
		// For regular files, seek directly
		if startPosition > 0 {
			_, err := file.Seek(startPosition, io.SeekStart)
			if err != nil {
				file.Close()
				return nil, fmt.Errorf("failed to seek to position %d: %w", startPosition, err)
			}
		}
		return file, nil
	}
}

// gzipReaderCloser wraps gzip.Reader to close both the gzip reader and underlying file
type gzipReaderCloser struct {
	gzReader *gzip.Reader
	file     *os.File
}

func (g *gzipReaderCloser) Read(p []byte) (n int, err error) {
	return g.gzReader.Read(p)
}

func (g *gzipReaderCloser) Close() error {
	if err := g.gzReader.Close(); err != nil {
		g.file.Close() // Still close file even if gzip reader fails
		return err
	}
	return g.file.Close()
}

// getAllLogGroups returns all unique log groups
func (rm *RebuildManager) getAllLogGroups() ([]string, error) {
	if rm.persistence == nil {
		return []string{}, nil
	}

	indexes, err := rm.persistence.GetAllLogIndexes()
	if err != nil {
		return nil, err
	}

	// Use map to get unique main log paths
	groups := make(map[string]struct{})
	for _, idx := range indexes {
		groups[idx.MainLogPath] = struct{}{}
	}

	// Convert to slice
	result := make([]string, 0, len(groups))
	for group := range groups {
		result = append(result, group)
	}

	return result, nil
}

// deleteAllIndexes deletes all existing indexes
func (rm *RebuildManager) deleteAllIndexes() error {
	// Get all shards
	shards := rm.shardManager.GetAllShards()

	// Delete each shard
	for i, shard := range shards {
		if shard != nil {
			if err := shard.Close(); err != nil {
				return fmt.Errorf("failed to close shard %d: %w", i, err)
			}
		}
	}

	// Recreate shards
	// This would typically be done by recreating the shard manager
	// For now, return nil as placeholder
	return nil
}

// deleteLogGroupIndex deletes index for a specific log group
func (rm *RebuildManager) deleteLogGroupIndex(logGroupPath string) error {
	// In a real implementation, this would:
	// 1. Find all documents for this log group
	// 2. Delete them from the appropriate shards
	// For now, return nil as placeholder
	return nil
}

// resetAllPersistenceRecords resets all persistence records
func (rm *RebuildManager) resetAllPersistenceRecords() error {
	if rm.persistence == nil {
		return nil
	}

	indexes, err := rm.persistence.GetAllLogIndexes()
	if err != nil {
		return err
	}

	for _, idx := range indexes {
		idx.Reset()
		if err := rm.persistence.SaveLogIndex(idx); err != nil {
			return fmt.Errorf("failed to reset index %s: %w", idx.Path, err)
		}
	}

	return nil
}

// resetLogGroupPersistence resets persistence for a log group
func (rm *RebuildManager) resetLogGroupPersistence(logGroupPath string) error {
	if rm.persistence == nil {
		return nil
	}

	indexes, err := rm.persistence.GetLogGroupIndexes(logGroupPath)
	if err != nil {
		return err
	}

	for _, idx := range indexes {
		idx.Reset()
		if err := rm.persistence.SaveLogIndex(idx); err != nil {
			return fmt.Errorf("failed to reset index %s: %w", idx.Path, err)
		}
	}

	return nil
}

// RebuildProgress tracks rebuild progress
type RebuildProgress struct {
	TotalGroups     int
	CompletedGroups int
	StartTime       time.Time
	CompletedTime   time.Time
	Duration        time.Duration
	CurrentGroup    string
	CurrentFile     string
	Errors          []error
}

// notification methods
func (rm *RebuildManager) notifyRebuildProgress(progress *RebuildProgress) {
	// Emit progress event
	// This would typically publish to an event bus
}

func (rm *RebuildManager) notifyRebuildComplete(progress *RebuildProgress) {
	// Emit completion event
}

func (rm *RebuildManager) notifySingleRebuildComplete(logGroupPath string, duration time.Duration) {
	// Emit single rebuild completion event
}

func (rm *RebuildManager) handleProgressNotification(logGroupPath string, pn ProgressNotification) {
	// Handle progress notification from tracker
}

func (rm *RebuildManager) handleCompletionNotification(logGroupPath string, cn CompletionNotification) {
	// Handle completion notification from tracker
}

// IsRebuilding returns true if rebuild is in progress
func (rm *RebuildManager) IsRebuilding() bool {
	return atomic.LoadInt32(&rm.rebuilding) == 1
}

// GetLastRebuildTime returns the time of the last rebuild
func (rm *RebuildManager) GetLastRebuildTime() time.Time {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.lastRebuildTime
}

// RebuildStats GetRebuildStats returns statistics about rebuild operations
type RebuildStats struct {
	IsRebuilding    bool           `json:"is_rebuilding"`
	LastRebuildTime time.Time      `json:"last_rebuild_time"`
	Config          *RebuildConfig `json:"config"`
}

func (rm *RebuildManager) GetRebuildStats() *RebuildStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return &RebuildStats{
		IsRebuilding:    rm.IsRebuilding(),
		LastRebuildTime: rm.lastRebuildTime,
		Config:          rm.config,
	}
}
