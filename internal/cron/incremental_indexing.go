package cron

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-co-op/gocron/v2"
	"github.com/uozi-tech/cosy/logger"
)

// logIndexProvider provides access to stored per-file index metadata.
type logIndexProvider interface {
	GetLogIndex(path string) (*model.NginxLogIndex, error)
}

// setupIncrementalIndexingJob sets up the periodic incremental log indexing job
func setupIncrementalIndexingJob(s gocron.Scheduler) (gocron.Job, error) {
	logger.Info("Setting up incremental log indexing job")

	// Determine interval from settings, falling back to a conservative default
	interval := settings.NginxLogSettings.GetIncrementalIndexInterval()

	// Run periodically to check for log file changes using incremental indexing
	job, err := s.NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(performIncrementalIndexing),
		gocron.WithName("incremental_log_indexing"),
		gocron.WithSingletonMode(gocron.LimitModeWait), // Prevent overlapping executions
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)

	if err != nil {
		return nil, err
	}

	logger.Infof("Incremental log indexing job scheduled to run every %s", interval)
	return job, nil
}

// performIncrementalIndexing performs the actual incremental indexing check
func performIncrementalIndexing() {
	logger.Debug("Starting incremental log indexing scan")

	// Get log file manager
	logFileManager := nginx_log.GetLogFileManager()
	if logFileManager == nil {
		logger.Warn("Log file manager not available for incremental indexing")
		return
	}

	persistence := logFileManager.GetPersistence()
	if persistence == nil {
		logger.Warn("Persistence manager not available for incremental indexing")
		return
	}

	// Get modern indexer
	modernIndexer := nginx_log.GetIndexer()
	if modernIndexer == nil {
		logger.Warn("Modern indexer not available for incremental indexing")
		return
	}

	// Check if indexer is healthy
	if !modernIndexer.IsHealthy() {
		logger.Warn("Modern indexer is not healthy, skipping incremental indexing")
		return
	}

	// Get all log groups to check for changes
	allLogs := nginx_log.GetAllLogsWithIndexGrouped(func(log *nginx_log.NginxLogWithIndex) bool {
		// Only process access logs (skip error logs as they are not indexed)
		return log.Type == "access"
	})

	// Process files sequentially to avoid overwhelming the system
	// This is more conservative but prevents concurrent file indexing from consuming too much CPU
	changedCount := 0
	for _, log := range allLogs {
		// Check if file needs incremental indexing
		if needsIncrementalIndexing(log, persistence) {
			logger.Debugf("Starting incremental indexing for file: %s", log.Path)

			// Set status to indexing
			if err := setFileIndexStatus(log.Path, string(indexer.IndexStatusIndexing), logFileManager); err != nil {
				logger.Errorf("Failed to set indexing status for %s: %v", log.Path, err)
				continue
			}

			// Perform incremental indexing synchronously (one file at a time)
			if err := performSingleFileIncrementalIndexing(log.Path, modernIndexer, logFileManager); err != nil {
				logger.Errorf("Failed incremental indexing for %s: %v", log.Path, err)
				// Set error status
				if statusErr := setFileIndexStatus(log.Path, string(indexer.IndexStatusError), logFileManager); statusErr != nil {
					logger.Errorf("Failed to set error status for %s: %v", log.Path, statusErr)
				}
			} else {
				changedCount++
				// Set status to indexed
				if err := setFileIndexStatus(log.Path, string(indexer.IndexStatusIndexed), logFileManager); err != nil {
					logger.Errorf("Failed to set indexed status for %s: %v", log.Path, err)
				}
			}
		}
	}

	if changedCount > 0 {
		logger.Debugf("Completed incremental indexing for %d log files", changedCount)
		// Update searcher shards once after all files are processed
		nginx_log.UpdateSearcherShards()
	} else {
		logger.Debug("No log files need incremental indexing")
	}
}

// needsIncrementalIndexing checks if a log file needs incremental indexing
func needsIncrementalIndexing(log *nginx_log.NginxLogWithIndex, persistence logIndexProvider) bool {
	// Skip if already indexing or queued
	if log.IndexStatus == string(indexer.IndexStatusIndexing) ||
		log.IndexStatus == string(indexer.IndexStatusQueued) {
		return false
	}

	// Check file system status
	fileInfo, err := os.Stat(log.Path)
	if os.IsNotExist(err) {
		// File doesn't exist, but we have index data - this is fine for historical queries
		return false
	}
	if err != nil {
		logger.Warnf("Cannot stat file %s: %v", log.Path, err)
		return false
	}

	fileModTime := fileInfo.ModTime()
	fileSize := fileInfo.Size()

	// CRITICAL FIX: For large files (>100MB), add additional check to prevent excessive re-indexing
	// If the file was recently indexed (within last 30 minutes), skip it even if size increased slightly
	// This prevents the "infinite indexing" issue reported in #1455
	const largeFileThreshold = 100 * 1024 * 1024 // 100MB
	const recentIndexThreshold = 30 * time.Minute

	if fileSize > largeFileThreshold && log.LastIndexed > 0 {
		lastIndexTime := time.Unix(log.LastIndexed, 0)
		timeSinceLastIndex := time.Since(lastIndexTime)

		if timeSinceLastIndex < recentIndexThreshold {
			logger.Debugf("Skipping large file %s (%d bytes): recently indexed %v ago (threshold: %v)",
				log.Path, fileSize, timeSinceLastIndex, recentIndexThreshold)
			return false
		}
	}

	if persistence != nil {
		if logIndex, err := persistence.GetLogIndex(log.Path); err == nil {
			if logIndex.NeedsIndexing(fileModTime, fileSize) {
				logger.Debugf("File %s needs incremental indexing based on persisted metadata", log.Path)
				return true
			}
			return false
		} else {
			logger.Debugf("Could not load persisted metadata for %s: %v", log.Path, err)
		}
	}

	// Fallback: use aggregated data cautiously by clamping the stored size so grouped entries
	// do not trigger false positives when rotation files are aggregated together.
	lastModified := time.Unix(log.LastModified, 0)
	rawLastSize := log.LastSize
	clampedLastSize := rawLastSize
	if clampedLastSize == 0 {
		clampedLastSize = fileSize
	} else if clampedLastSize > fileSize {
		clampedLastSize = fileSize
	}

	// If the file was never indexed, queue it once.
	if log.LastIndexed == 0 {
		return true
	}

	if fileModTime.After(lastModified) && fileSize > clampedLastSize {
		logger.Debugf("File %s needs incremental indexing (fallback path): mod_time=%s, size=%d",
			log.Path, fileModTime.Format("2006-01-02 15:04:05"), fileSize)
		return true
	}

	if fileSize < clampedLastSize {
		logger.Debugf("File %s needs full re-indexing (fallback path) due to size decrease: old_size=%d (raw=%d), new_size=%d",
			log.Path, clampedLastSize, rawLastSize, fileSize)
		return true
	}

	return false
}

// performSingleFileIncrementalIndexing performs TRUE incremental indexing for a single file synchronously
// This implements real incremental indexing by using LastPosition to only read new content
func performSingleFileIncrementalIndexing(logPath string, modernIndexer interface{}, logFileManager interface{}) error {
	defer func() {
		// Ensure status is always updated, even on panic
		if r := recover(); r != nil {
			logger.Errorf("Recovered from panic during incremental indexing for %s: %v", logPath, r)
			_ = setFileIndexStatus(logPath, string(indexer.IndexStatusError), logFileManager)
		}
	}()

	lfm, ok := logFileManager.(*indexer.LogFileManager)
	if !ok {
		return fmt.Errorf("invalid log file manager type")
	}

	persistence := lfm.GetPersistence()
	if persistence == nil {
		return fmt.Errorf("persistence not available")
	}

	// Get current file info
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	currentSize := fileInfo.Size()
	isGzipped := strings.HasSuffix(strings.ToLower(logPath), ".gz")

	// Check existing index metadata
	existingIndex, err := persistence.GetLogIndex(logPath)
	if err != nil {
		logger.Warnf("Could not get existing log index for %s: %v", logPath, err)
	}

	var startPosition int64 = 0
	var existingDocCount uint64 = 0

	if existingIndex != nil {
		if isGzipped {
			// For gzip files, we cannot reliably map persisted LastPosition (compressed bytes)
			// to a position in the decompressed stream. Treat every incremental run as a
			// full re-index when the file changes to avoid skipping or duplicating data.
			logger.Debugf("Gzip file %s detected; ignoring LastPosition and resetting document count for full re-index (last_size=%d, current_size=%d)",
				logPath, existingIndex.LastSize, currentSize)
			startPosition = 0
			existingDocCount = 0
		} else {
			existingDocCount = existingIndex.DocumentCount

			// Detect file rotation (size decreased)
			if currentSize < existingIndex.LastSize {
				startPosition = 0
				existingDocCount = 0 // Reset count for rotated file
				logger.Debugf("Log rotation detected for %s: size %d -> %d, full re-index",
					logPath, existingIndex.LastSize, currentSize)
			} else if existingIndex.LastPosition > 0 && existingIndex.LastPosition < currentSize {
				// TRUE INCREMENTAL: File grew, resume from last position
				startPosition = existingIndex.LastPosition
				logger.Debugf("TRUE INCREMENTAL: %s grew %d -> %d bytes, reading from position %d",
					logPath, existingIndex.LastSize, currentSize, startPosition)
			} else if existingIndex.LastPosition == currentSize {
				// File unchanged
				logger.Debugf("File %s unchanged (size=%d, position=%d), skipping",
					logPath, currentSize, existingIndex.LastPosition)
				return nil
			} else if existingIndex.LastPosition == 0 && existingDocCount > 0 {
				// Inconsistent state: we have documents but no recorded position.
				// Treat this as a full re-index from the beginning to avoid duplicate counting.
				logger.Debugf("Inconsistent index state for %s (docs=%d, last_position=0); resetting existing count and re-indexing from start",
					logPath, existingDocCount)
				startPosition = 0
				existingDocCount = 0
			}
		}
	}

	// Perform incremental indexing with position-aware reading
	startTime := time.Now()
	newDocsIndexed, minTime, maxTime, finalPosition, err := indexFileFromPosition(
		modernIndexer.(*indexer.ParallelIndexer),
		logPath,
		startPosition,
	)

	if err != nil {
		return fmt.Errorf("indexing failed: %w", err)
	}

	duration := time.Since(startTime)
	finalDocCount := existingDocCount + newDocsIndexed

	// Save metadata with updated position
	if err := lfm.SaveIndexMetadata(logPath, finalDocCount, startTime, duration, minTime, maxTime); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// CRITICAL FIX for Bug 1 & Bug 2:
	// Re-fetch the index record after SaveIndexMetadata to ensure we have the latest data
	// (SaveIndexMetadata internally updates LastModified and other fields)
	// Then update LastPosition which is critical for true incremental indexing
	updatedIndex, err := persistence.GetLogIndex(logPath)
	if err != nil {
		// If we still can't get it, this is a critical error as LastPosition won't be persisted
		return fmt.Errorf("failed to get index after save (LastPosition will be lost): %w", err)
	}

	// Get the CURRENT file info again to ensure we record the latest size and modification time
	finalFileInfo, err := os.Stat(logPath)
	if err != nil {
		return fmt.Errorf("failed to stat file after indexing: %w", err)
	}

	// Update position to the end of the data we actually read.
	// For non-gzip files, this is the byte offset returned by indexFileFromPosition.
	// For gzip files, LastPosition is not used for incremental seeks and is kept for diagnostics only.
	updatedIndex.LastPosition = finalPosition
	updatedIndex.LastSize = finalFileInfo.Size()
	updatedIndex.LastModified = finalFileInfo.ModTime()

	if err := persistence.SaveLogIndex(updatedIndex); err != nil {
		return fmt.Errorf("failed to update LastPosition (incremental will fail next time): %w", err)
	}

	logger.Debugf("TRUE INCREMENTAL completed: %s, new_docs=%d, total_docs=%d, position=%d->%d",
		logPath, newDocsIndexed, finalDocCount, startPosition, finalFileInfo.Size())
	return nil
}

// indexFileFromPosition reads and indexes only the new content from a file starting at the given position.
// This is the core implementation of TRUE incremental indexing.
// It returns the number of successfully indexed documents, the time range, and the final byte position read.
func indexFileFromPosition(pi *indexer.ParallelIndexer, filePath string, startPosition int64) (uint64, *time.Time, *time.Time, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, nil, nil, 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return 0, nil, nil, 0, fmt.Errorf("failed to stat file: %w", err)
	}

	fileSize := fileInfo.Size()
	isGzipped := strings.HasSuffix(strings.ToLower(filePath), ".gz")
	var reader io.Reader

	if isGzipped {
		// Gzip files: must read from beginning and discard up to startPosition
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return 0, nil, nil, 0, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()

		if startPosition > 0 {
			// WARNING: For large gzip files, this is still slow as we must decompress from start
			// Consider skipping gzip files that were recently indexed
			logger.Debugf("Gzip %s: reading %d bytes to skip to position %d", filePath, startPosition, startPosition)
			if _, err := io.CopyN(io.Discard, gzReader, startPosition); err != nil && err != io.EOF {
				return 0, nil, nil, 0, fmt.Errorf("failed to skip to position: %w", err)
			}
		}
		reader = gzReader
	} else {
		// Regular files: direct seek (fast!)
		if startPosition > 0 {
			if _, err := file.Seek(startPosition, io.SeekStart); err != nil {
				return 0, nil, nil, 0, fmt.Errorf("failed to seek: %w", err)
			}
			logger.Debugf("Seeked to position %d in %s (file size: %d, reading %d new bytes)",
				startPosition, filePath, fileSize, fileSize-startPosition)
		}
		reader = file
	}

	// Parse only the new content
	ctx := context.Background()
	logDocs, err := indexer.ParseLogStream(ctx, reader, filePath)
	if err != nil {
		return 0, nil, nil, 0, fmt.Errorf("failed to parse new content: %w", err)
	}

	// Calculate time range for new documents using stable values
	var (
		minTimeVal time.Time
		maxTimeVal time.Time
		hasMin     bool
		hasMax     bool
	)
	for _, doc := range logDocs {
		if doc.Timestamp <= 0 {
			continue
		}
		ts := time.Unix(doc.Timestamp, 0)
		if !hasMin || ts.Before(minTimeVal) {
			minTimeVal = ts
			hasMin = true
		}
		if !hasMax || ts.After(maxTimeVal) {
			maxTimeVal = ts
			hasMax = true
		}
	}

	var minTime, maxTime *time.Time
	if hasMin {
		minTime = &minTimeVal
	}
	if hasMax {
		maxTime = &maxTimeVal
	}

	// Index the new documents using batch writer
	var indexedDocCount uint64
	var finalPosition int64

	// CRITICAL: Calculate finalPosition BEFORE batch operations to ensure it's available
	// even if batch.Flush() fails. This prevents losing track of where we read to.
	// Bug fix for issue where flush failure returns position=0, causing duplicate indexing.
	if !isGzipped {
		// For regular files, get current file position after ParseLogStream finished reading
		if pos, err := file.Seek(0, io.SeekCurrent); err == nil {
			finalPosition = pos
		} else {
			logger.Warnf("Failed to determine current read position for %s: %v", filePath, err)
			// Fallback: assume we read to EOF if we can't get position
			finalPosition = fileSize
		}
	} else {
		// For gzip files, we've decompressed the entire stream to EOF
		// LastPosition is not used for seeks but kept for diagnostics
		finalPosition = fileSize
	}

	if len(logDocs) > 0 {
		batch := pi.StartBatch()

		for i, doc := range logDocs {
			// Deterministic, segment-scoped document ID:
			// - filePath: physical log file
			// - startPosition: byte offset where this incremental segment begins
			// - i: index within this segment
			// This ensures:
			//   * Uniqueness within a single run
			//   * Stable IDs across retries for the same (filePath, startPosition) segment,
			//     so re-processing due to errors overwrites instead of creating duplicates.
			docID := fmt.Sprintf("%s_%d_%d", filePath, startPosition, i)

			document := &indexer.Document{
				ID:     docID,
				Fields: doc,
			}
			if err := batch.Add(document); err != nil {
				// If Add fails, an auto-flush may have failed internally. We conservatively
				// treat this document as not indexed and continue with the remaining ones.
				logger.Warnf("Failed to add document %s: %v", docID, err)
				continue
			}
			indexedDocCount++
		}

		// At this point:
		//   indexedDocCount = total documents successfully handed to the batch writer
		//   batch.Size()    = documents currently buffered but NOT yet flushed.
		// Any documents that were auto-flushed due to internal batch limits have already
		// been sent to the indexer and removed from the internal buffer.
		pendingBeforeFlush := batch.Size()
		autoFlushedCount := indexedDocCount
		if pendingBeforeFlush > 0 && indexedDocCount >= uint64(pendingBeforeFlush) {
			autoFlushedCount = indexedDocCount - uint64(pendingBeforeFlush)
		}

		if _, err := batch.Flush(); err != nil {
			// CRITICAL BUG FIX: Return the actual finalPosition we calculated earlier,
			// not 0. This ensures that even on flush failure, the next incremental run
			// knows where we read to and won't duplicate the auto-flushed documents.
			logger.Warnf("Final batch flush failed for %s: %v (auto-flushed docs=%d, pending=%d, position will be saved as=%d)",
				filePath, err, autoFlushedCount, pendingBeforeFlush, finalPosition)
			return autoFlushedCount, minTime, maxTime, finalPosition, fmt.Errorf("failed to flush batch: %w", err)
		}
	}

	logger.Debugf("Indexed %d NEW documents from %s (position %d -> %d)",
		indexedDocCount, filePath, startPosition, fileSize)

	return indexedDocCount, minTime, maxTime, finalPosition, nil
}

// setFileIndexStatus updates the index status for a file in the database using enhanced status management
func setFileIndexStatus(logPath, status string, logFileManager interface{}) error {
	if logFileManager == nil {
		return fmt.Errorf("log file manager not available")
	}

	// Get persistence manager
	lfm, ok := logFileManager.(*indexer.LogFileManager)
	if !ok {
		return fmt.Errorf("invalid log file manager type")
	}
	persistence := lfm.GetPersistence()
	if persistence == nil {
		return fmt.Errorf("persistence manager not available")
	}

	// Use enhanced SetIndexStatus method with queue position for queued status
	queuePosition := 0
	if status == string(indexer.IndexStatusQueued) {
		// For incremental indexing, we don't need specific queue positions
		// They will be processed as they come
		queuePosition = int(time.Now().Unix() % 1000) // Simple ordering by time
	}

	return persistence.SetIndexStatus(logPath, status, queuePosition, "")
}
