package nginx_log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/blevesearch/bleve/v2"
	"github.com/uozi-tech/cosy/logger"
)

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
		li.logPaths[path].LastModified = 0
		li.logPaths[path].LastSize = 0
		li.logPaths[path].LastIndexed = 0
		li.logPaths[path].TimeRange = nil // Clear in-memory time range
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

	// Clear all log group completion flags to allow new notifications
	li.logGroupCompletionSent.Range(func(key, value interface{}) bool {
		li.logGroupCompletionSent.Delete(key)
		return true
	})

	// --- Start of Synchronous Re-indexing Logic ---
	logger.Infof("Starting synchronous re-indexing of all discovered files")

	// 1. Discover all potential log files from the main access log path
	var discoveredFiles []string
	accessLogPath := nginx.GetAccessLogPath()
	if accessLogPath != "" && IsLogPathUnderWhiteList(accessLogPath) {
		logDir := filepath.Dir(accessLogPath)
		baseLogName := filepath.Base(accessLogPath)
		files, err := li.findRelatedLogFiles(logDir, baseLogName)
		if err != nil {
			logger.Errorf("Failed to discover log files, proceeding with tracked files only: %v", err)
		} else {
			discoveredFiles = files
			logger.Infof("Discovered %d log files to consider for re-indexing", len(discoveredFiles))
		}
	}

	// 2. Get all currently tracked files and discover all log groups
	li.mu.RLock()
	allLogGroups := make(map[string]struct{})
	for path := range li.logPaths {
		mainLogPath := li.getMainLogPath(path)
		allLogGroups[mainLogPath] = struct{}{}
	}
	li.mu.RUnlock()
	// Also add discovered files to the set of log groups
	for _, path := range discoveredFiles {
		mainLogPath := li.getMainLogPath(path)
		allLogGroups[mainLogPath] = struct{}{}
	}

	// 3. Create WaitGroup and dispatch tasks for each unique log group
	var wg sync.WaitGroup
	logger.Infof("Queueing re-index tasks for %d unique log groups", len(allLogGroups))
	for mainLogPath := range allLogGroups {
		// ForceReindexFileGroup will discover files, add to WaitGroup, and queue tasks internally
		if err := li.ForceReindexFileGroup(mainLogPath, &wg); err != nil {
			logger.Warnf("Failed to queue force reindex for log group %s: %v", mainLogPath, err)
		}
	}

	// 4. Wait for all indexing tasks to complete
	logger.Infof("Waiting for all re-indexing tasks for log groups to complete...")
	wg.Wait()

	// 5. Completion notifications are now handled automatically by ProgressTracker
	// when each log group finishes indexing - no need for manual notification
	logger.Infof("All log groups have completed indexing - notifications sent automatically")

	// 6. Finalize status
	statusManager.UpdateIndexingStatus()

	logger.Infof("Synchronous re-indexing completed for all queued files")
	logger.Infof("Index rebuild completed successfully")
	return nil
}

// DiscoverAndIndexFile discovers and indexes a single log file
func (li *LogIndexer) DiscoverAndIndexFile(filePath string, lastModified time.Time, lastSize int64) error {
	logDir := filepath.Dir(filePath)
	baseLogName := filepath.Base(filePath)

	return li.DiscoverLogFiles(logDir, baseLogName)
}

// DeleteFileIndex removes all index entries for a specific file
func (li *LogIndexer) DeleteFileIndex(filePath string) error {
	logger.Infof("Deleting index entries for file: %s", filePath)

	// Create query to find all entries for this file
	query := bleve.NewTermQuery(filePath)
	query.SetField("file_path")
	searchReq := bleve.NewSearchRequest(query)
	searchReq.Size = 10000 // Process in batches

	totalDeleted := 0
	for {
		searchResult, err := li.index.Search(searchReq)
		if err != nil {
			return fmt.Errorf("failed to search for entries to delete: %w", err)
		}

		if len(searchResult.Hits) == 0 {
			break
		}

		// Create batch deletion
		batch := li.index.NewBatch()
		for _, hit := range searchResult.Hits {
			batch.Delete(hit.ID)
		}

		if err := li.index.Batch(batch); err != nil {
			return fmt.Errorf("failed to delete entries: %w", err)
		}

		totalDeleted += len(searchResult.Hits)
		logger.Infof("Deleted %d entries from %s (total: %d)", len(searchResult.Hits), filePath, totalDeleted)
	}

	// Remove from tracking
	li.mu.Lock()
	delete(li.logPaths, filePath)
	li.mu.Unlock()

	// Remove from persistence
	if li.persistence != nil {
		if err := li.persistence.DeleteLogIndex(filePath); err != nil {
			logger.Warnf("Failed to delete persistence record for %s: %v", filePath, err)
		}
	}

	// Clear related caches
	li.invalidateStatsCache()

	logger.Infof("Successfully deleted %d index entries for %s", totalDeleted, filePath)
	return nil
}

// DeleteAllIndexes removes all index entries
func (li *LogIndexer) DeleteAllIndexes() error {
	logger.Info("Deleting all index entries...")

	// Close current index
	if err := li.index.Close(); err != nil {
		logger.Warnf("Failed to close index during deletion: %v", err)
	}

	// Remove index directory
	if err := os.RemoveAll(li.indexPath); err != nil {
		return fmt.Errorf("failed to remove index directory: %w", err)
	}

	// Create new empty index
	mapping := createIndexMapping()
	index, err := bleve.New(li.indexPath, mapping)
	if err != nil {
		return fmt.Errorf("failed to create new empty index: %w", err)
	}
	li.index = index

	// Clear all tracking
	li.mu.Lock()
	li.logPaths = make(map[string]*LogFileInfo)
	li.mu.Unlock()

	// Clear persistence
	if li.persistence != nil {
		if err := li.persistence.DeleteAllLogIndexes(); err != nil {
			logger.Warnf("Failed to clear persistence: %v", err)
		}
	}

	// Clear caches
	li.cache.Clear()
	li.statsCache.Clear()

	logger.Info("Successfully deleted all index entries")
	return nil
}

// CleanupOrphanedIndexes removes index entries for files that no longer exist
func (li *LogIndexer) CleanupOrphanedIndexes() error {
	logger.Info("Cleaning up orphaned index entries...")

	li.mu.RLock()
	paths := make([]string, 0, len(li.logPaths))
	for path := range li.logPaths {
		paths = append(paths, path)
	}
	li.mu.RUnlock()

	orphanedPaths := make([]string, 0)

	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			orphanedPaths = append(orphanedPaths, path)
		}
	}

	if len(orphanedPaths) == 0 {
		logger.Info("No orphaned index entries found")
		return nil
	}

	logger.Infof("Found %d orphaned files to clean up: %v", len(orphanedPaths), orphanedPaths)

	for _, path := range orphanedPaths {
		if err := li.DeleteFileIndex(path); err != nil {
			logger.Errorf("Failed to delete orphaned index for %s: %v", path, err)
		} else {
			logger.Infof("Cleaned up orphaned index for %s", path)
		}
	}

	logger.Infof("Cleanup completed for %d orphaned files", len(orphanedPaths))
	return nil
}