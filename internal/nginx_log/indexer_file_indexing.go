package nginx_log

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

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

// IndexLogFileIncremental performs incremental indexing of a log file
func (li *LogIndexer) IndexLogFileIncremental(filePath string) error {
	logger.Infof("Starting incremental index of log file: %s", filePath)

	// Note: Global indexing status is managed at the rebuild level
	// Individual file notifications are not needed as they cause excessive status changes
	defer SetIndexingStatus(filePath, false) // Clear individual file status when done

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

// ForceReindexFileGroup cleans, discovers, and queues all files for a log group.
func (li *LogIndexer) ForceReindexFileGroup(mainLogPath string, wg *sync.WaitGroup) error {
	logger.Infof("Force reindexing log group: %s", mainLogPath)

	// 1. Delete all existing index data for this entire log group
	if err := li.DeleteLogGroupFromIndex(mainLogPath); err != nil {
		return fmt.Errorf("failed to delete log group %s before reindexing: %w", mainLogPath, err)
	}

	// 2. Discover all files belonging to this log group
	logDir := filepath.Dir(mainLogPath)
	baseLogName := filepath.Base(mainLogPath)
	relatedFiles, err := li.findRelatedLogFiles(logDir, baseLogName)
	if err != nil {
		return fmt.Errorf("failed to find related files for log group %s: %w", mainLogPath, err)
	}

	logger.Infof("Found %d files to reindex for log group %s", len(relatedFiles), mainLogPath)

	// 3. Clear completion flag to allow new notifications
	li.clearLogGroupCompletionFlag(mainLogPath)

	// 4. Record log group start time (use the main log path to store group-level timing)
	groupStartTime := time.Now()
	if mainLogIndex, err := li.persistence.GetLogIndex(mainLogPath); err == nil {
		mainLogIndex.SetIndexStartTime(groupStartTime)
		if err := li.persistence.SaveLogIndex(mainLogIndex); err != nil {
			logger.Warnf("Failed to save group start time for %s: %v", mainLogPath, err)
		}
	}

	// 5. Reset persistence for all files and queue a single task for the entire log group
	for _, file := range relatedFiles {
		if logIndex, err := li.persistence.GetLogIndex(file); err == nil {
			logIndex.Reset()
			if err := li.persistence.SaveLogIndex(logIndex); err != nil {
				logger.Warnf("Failed to reset persistence for %s: %v", file, err)
			}
		}
		SetIndexingStatus(file, true)
	}

	// Queue a single task for the entire log group (not per file)
	wg.Add(1)
	li.queueIndexTask(&IndexTask{
		FilePath:    mainLogPath, // Use main log path to represent the entire group
		Priority:    10,
		FullReindex: true,
		Wg:          wg,
	}, wg)

	statusManager := GetIndexingStatusManager()
	statusManager.UpdateIndexingStatus()

	return nil
}