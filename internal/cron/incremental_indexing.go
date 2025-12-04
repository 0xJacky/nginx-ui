package cron

import (
	"fmt"
	"os"
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
			logger.Infof("Starting incremental indexing for file: %s", log.Path)

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
		logger.Infof("Completed incremental indexing for %d log files", changedCount)
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

// performSingleFileIncrementalIndexing performs incremental indexing for a single file synchronously
func performSingleFileIncrementalIndexing(logPath string, modernIndexer interface{}, logFileManager interface{}) error {
	defer func() {
		// Ensure status is always updated, even on panic
		if r := recover(); r != nil {
			logger.Errorf("Recovered from panic during incremental indexing for %s: %v", logPath, r)
			_ = setFileIndexStatus(logPath, string(indexer.IndexStatusError), logFileManager)
		}
	}()

	// Perform incremental indexing
	startTime := time.Now()
	docsCountMap, minTime, maxTime, err := modernIndexer.(*indexer.ParallelIndexer).IndexSingleFileIncrementally(logPath, nil)

	if err != nil {
		return fmt.Errorf("indexing failed: %w", err)
	}

	// Calculate total documents indexed
	var totalDocsIndexed uint64
	for _, docCount := range docsCountMap {
		totalDocsIndexed += docCount
	}

	// Save indexing metadata
	duration := time.Since(startTime)

	if lfm, ok := logFileManager.(*indexer.LogFileManager); ok {
		persistence := lfm.GetPersistence()
		var existingDocCount uint64

		existingIndex, err := persistence.GetLogIndex(logPath)
		if err != nil {
			logger.Warnf("Could not get existing log index for %s: %v", logPath, err)
		}

		// Determine if the file was rotated by checking if the current size is smaller than the last recorded size.
		// This is a strong indicator of log rotation.
		fileInfo, statErr := os.Stat(logPath)
		isRotated := false
		if statErr == nil && existingIndex != nil && fileInfo.Size() < existingIndex.LastSize {
			isRotated = true
			logger.Infof("Log rotation detected for %s: new size %d is smaller than last size %d. Resetting document count.",
				logPath, fileInfo.Size(), existingIndex.LastSize)
		}

		if existingIndex != nil && !isRotated {
			// If it's a normal incremental update (not a rotation), we build upon the existing count.
			existingDocCount = existingIndex.DocumentCount
		}
		// If the file was rotated, existingDocCount remains 0, effectively starting the count over for the new file.

		finalDocCount := existingDocCount + totalDocsIndexed

		if err := lfm.SaveIndexMetadata(logPath, finalDocCount, startTime, duration, minTime, maxTime); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}
	}

	logger.Infof("Successfully completed incremental indexing for %s, Documents: %d", logPath, totalDocsIndexed)
	return nil
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
