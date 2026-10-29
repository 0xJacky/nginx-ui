package cron

import (
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
	if !shouldRunIncrementalIndexing() {
		logger.Debug("Advanced log indexing is disabled; skipping incremental log indexing scan")
		return
	}

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
	scheduler := nginx_log.GetTaskScheduler()
	changedCount := 0
	for _, log := range allLogs {
		// Check if file needs incremental indexing
		if !needsIncrementalIndexing(log, persistence) {
			continue
		}

		indexed := false
		run := func() {
			indexed = indexLogFileIncrementally(log.Path, modernIndexer, logFileManager)
		}

		// Run under the scheduler's group lock and run slot, so this job
		// neither races a scheduled task on the same log group nor raises the
		// number of groups being indexed at once.
		if scheduler != nil {
			if !scheduler.TryRunIndexTask(context.Background(), log.Path, run) {
				logger.Debugf("Skipping incremental indexing for %s: an indexing task is running for it", log.Path)
				continue
			}
		} else {
			run()
		}

		if indexed {
			changedCount++
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

// indexLogFileIncrementally indexes one log file and keeps its index status in
// step. It reports whether the file was indexed successfully.
func indexLogFileIncrementally(logPath string, modernIndexer *indexer.ParallelIndexer, logFileManager *indexer.LogFileManager) bool {
	logger.Debugf("Starting incremental indexing for file: %s", logPath)

	if err := setFileIndexStatus(logPath, string(indexer.IndexStatusIndexing), logFileManager); err != nil {
		logger.Errorf("Failed to set indexing status for %s: %v", logPath, err)
		return false
	}

	if err := performSingleFileIncrementalIndexing(logPath, modernIndexer, logFileManager); err != nil {
		logger.Errorf("Failed incremental indexing for %s: %v", logPath, err)
		if statusErr := setFileIndexStatus(logPath, string(indexer.IndexStatusError), logFileManager); statusErr != nil {
			logger.Errorf("Failed to set error status for %s: %v", logPath, statusErr)
		}
		return false
	}

	if err := setFileIndexStatus(logPath, string(indexer.IndexStatusIndexed), logFileManager); err != nil {
		logger.Errorf("Failed to set indexed status for %s: %v", logPath, err)
	}
	return true
}

func shouldRunIncrementalIndexing() bool {
	return settings.NginxLogSettings.IndexingEnabled
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

// incrementalMode is how an incremental run brings a file's index up to date.
type incrementalMode int

const (
	// incrementalSkip: the index already covers the whole file.
	incrementalSkip incrementalMode = iota
	// incrementalAppend: read only the bytes appended since LastPosition.
	incrementalAppend
	// incrementalReindexFile: drop the file's documents and re-read it from 0.
	incrementalReindexFile
	// incrementalReindexGroup: the file shrank, so its earlier content now
	// lives in a rotated sibling; re-index the whole log group.
	incrementalReindexGroup
)

// planIncrementalRun decides how to index a file of currentSize bytes given
// its persisted metadata. The returned position is only meaningful for
// incrementalAppend.
func planIncrementalRun(existing *model.NginxLogIndex, currentSize int64, isGzipped bool) (incrementalMode, int64) {
	// Nothing recorded, or a gzip file: a persisted LastPosition counts
	// compressed bytes and cannot be mapped into the decompressed stream.
	if existing == nil || isGzipped {
		return incrementalReindexFile, 0
	}

	switch {
	case currentSize < existing.LastSize || existing.LastPosition > currentSize:
		// The file is shorter than what was indexed: it was rotated or
		// truncated, and the indexed lines moved to a rotated file.
		return incrementalReindexGroup, 0
	case existing.LastPosition == currentSize:
		return incrementalSkip, 0
	case existing.LastPosition > 0:
		return incrementalAppend, existing.LastPosition
	default:
		// No recorded position. Re-reading from 0 on top of documents that
		// are already indexed is what duplicated every line (issue #2054),
		// so the re-read replaces the file's documents instead.
		return incrementalReindexFile, 0
	}
}

// performSingleFileIncrementalIndexing performs TRUE incremental indexing for a single file synchronously
// This implements real incremental indexing by using LastPosition to only read new content
func performSingleFileIncrementalIndexing(logPath string, modernIndexer *indexer.ParallelIndexer, lfm *indexer.LogFileManager) (err error) {
	defer func() {
		// Ensure status is always updated, even on panic
		if r := recover(); r != nil {
			logger.Errorf("Recovered from panic during incremental indexing for %s: %v", logPath, r)
			err = fmt.Errorf("panic during incremental indexing: %v", r)
		}
	}()

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
		existingIndex = nil
	}

	mode, startPosition := planIncrementalRun(existingIndex, currentSize, isGzipped)
	switch mode {
	case incrementalSkip:
		logger.Debugf("File %s unchanged (size=%d), skipping", logPath, currentSize)
		return nil
	case incrementalReindexFile:
		logger.Debugf("Re-indexing %s from the start (size=%d)", logPath, currentSize)
		return reindexLogFile(logPath, modernIndexer, lfm)
	case incrementalReindexGroup:
		logger.Debugf("Log rotation detected for %s: size %d -> %d, re-indexing the log group",
			logPath, existingIndex.LastSize, currentSize)
		return reindexLogGroup(logPath, modernIndexer, lfm)
	}

	logger.Debugf("TRUE INCREMENTAL: %s grew %d -> %d bytes, reading from position %d",
		logPath, existingIndex.LastSize, currentSize, startPosition)

	// Perform incremental indexing with position-aware reading
	startTime := time.Now()
	newDocsIndexed, minTime, maxTime, finalPosition, err := indexFileFromPosition(modernIndexer, logPath, startPosition)
	if err != nil {
		return fmt.Errorf("indexing failed: %w", err)
	}

	duration := time.Since(startTime)
	finalDocCount := existingIndex.DocumentCount + newDocsIndexed

	// Save metadata with updated position
	if err := lfm.SaveIndexMetadata(logPath, finalDocCount, startTime, duration, minTime, maxTime); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Re-fetch the index record after SaveIndexMetadata, which updates
	// LastModified and other fields, then record the new LastPosition.
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

	updatedIndex.LastPosition = finalPosition
	updatedIndex.LastSize = finalFileInfo.Size()
	updatedIndex.LastModified = finalFileInfo.ModTime()

	if err := persistence.SaveLogIndex(updatedIndex); err != nil {
		return fmt.Errorf("failed to update LastPosition (incremental will fail next time): %w", err)
	}

	logger.Debugf("TRUE INCREMENTAL completed: %s, new_docs=%d, total_docs=%d, position=%d->%d",
		logPath, newDocsIndexed, finalDocCount, startPosition, finalPosition)
	return nil
}

// reindexLogFile replaces a file's documents with a fresh full read of it.
// It goes through the same streaming path and document IDs as a scheduled
// full index, and SaveIndexMetadata records where that read stopped.
func reindexLogFile(logPath string, modernIndexer *indexer.ParallelIndexer, lfm *indexer.LogFileManager) error {
	if err := modernIndexer.DeleteIndexByFiles([]string{logPath}); err != nil {
		return fmt.Errorf("failed to delete existing documents before re-indexing: %w", err)
	}

	startTime := time.Now()
	docCount, minTime, maxTime, err := modernIndexer.IndexSingleFileWithProgress(logPath, nil)
	if err != nil {
		return fmt.Errorf("indexing failed: %w", err)
	}

	if err := lfm.SaveIndexMetadata(logPath, docCount, startTime, time.Since(startTime), minTime, maxTime); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}
	return nil
}

// reindexLogGroup replaces every document of a log group with a fresh index
// of its current files, the way a manual rebuild of the group does. After a
// rotation the documents of the live file describe what is now a rotated
// file, so only a group-wide re-index keeps them attributed correctly
// without duplicates.
func reindexLogGroup(logPath string, modernIndexer *indexer.ParallelIndexer, lfm *indexer.LogFileManager) error {
	if err := modernIndexer.DeleteIndexByMainLogPath(logPath); err != nil {
		return fmt.Errorf("failed to delete existing documents before re-indexing the log group: %w", err)
	}

	startTime := time.Now()
	docsCountMap, minTime, maxTime, err := modernIndexer.IndexLogGroupWithProgress(logPath, nil)
	if err != nil {
		return fmt.Errorf("indexing failed: %w", err)
	}
	duration := time.Since(startTime)

	var totalDocs uint64
	for _, docCount := range docsCountMap {
		totalDocs += docCount
	}
	if err := lfm.SaveIndexMetadata(logPath, totalDocs, startTime, duration, minTime, maxTime); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	for filePath, docCount := range docsCountMap {
		if filePath == logPath {
			continue
		}
		if err := lfm.SaveIndexMetadata(filePath, docCount, startTime, duration, minTime, maxTime); err != nil {
			logger.Errorf("Failed to save index metadata for %s: %v", filePath, err)
		}
	}
	return nil
}

// indexFileFromPosition reads and indexes only the new content of a plain
// (uncompressed) file, starting at the given byte position. The content is
// parsed and handed to the batch writer one parse batch at a time, so memory
// stays bounded however much was appended.
// It returns the number of successfully indexed documents, the time range, and the final byte position read.
func indexFileFromPosition(pi *indexer.ParallelIndexer, filePath string, startPosition int64) (uint64, *time.Time, *time.Time, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, nil, nil, 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if startPosition > 0 {
		if _, err := file.Seek(startPosition, io.SeekStart); err != nil {
			return 0, nil, nil, 0, fmt.Errorf("failed to seek: %w", err)
		}
	}

	var (
		indexedDocCount uint64
		segmentIndex    int
		minTime         *time.Time
		maxTime         *time.Time
	)

	batch := pi.StartBatch()
	_, _, err = indexer.ParseLogStreamBatches(context.Background(), file, filePath, func(docs []*indexer.LogDocument) error {
		for _, doc := range docs {
			// Deterministic, segment-scoped document ID: the file, the byte
			// offset this segment starts at, and the index within it. A retry
			// of the same segment overwrites instead of duplicating.
			docID := fmt.Sprintf("%s_%d_%d", filePath, startPosition, segmentIndex)
			segmentIndex++

			if doc.Timestamp > 0 {
				ts := time.Unix(doc.Timestamp, 0)
				if minTime == nil || ts.Before(*minTime) {
					minTime = &ts
				}
				if maxTime == nil || ts.After(*maxTime) {
					maxTime = &ts
				}
			}

			if err := batch.Add(&indexer.Document{ID: docID, Fields: doc}); err != nil {
				// If Add fails, an auto-flush may have failed internally. We conservatively
				// treat this document as not indexed and continue with the remaining ones.
				logger.Warnf("Failed to add document %s: %v", docID, err)
				continue
			}
			indexedDocCount++
		}
		return nil
	})
	if err != nil {
		return 0, nil, nil, 0, fmt.Errorf("failed to parse new content: %w", err)
	}

	// The parser read the file to EOF, so the descriptor offset is the end
	// of the data that was indexed.
	finalPosition, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, nil, nil, 0, fmt.Errorf("failed to determine read position: %w", err)
	}

	if _, err := batch.Flush(); err != nil {
		return 0, nil, nil, 0, fmt.Errorf("failed to flush batch: %w", err)
	}

	logger.Debugf("Indexed %d NEW documents from %s (position %d -> %d)",
		indexedDocCount, filePath, startPosition, finalPosition)

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
