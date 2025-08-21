package nginx_log

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/uozi-tech/cosy/logger"
)

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

	// Check if this path is already being tracked
	if existingInfo, exists := li.logPaths[logPath]; exists {
		// Update compressed status but keep existing tracking info
		existingInfo.IsCompressed = isCompressed
		logger.Debugf("Log path %s already being tracked, updated compressed status to %v", logPath, isCompressed)
	} else {
		// Add new log path with zero time to trigger initial indexing check
		li.logPaths[logPath] = &LogFileInfo{
			Path:         logPath,
			LastModified: 0, // Will trigger indexing check on first scan
			LastSize:     0, // Will trigger indexing check on first scan
			IsCompressed: isCompressed,
		}
		logger.Infof("Added new log path %s (compressed=%v)", logPath, isCompressed)
	}

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
	}, nil) // No waitgroup for regular add

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

// queueIndexTask adds a task to the indexing queue with debouncing
func (li *LogIndexer) queueIndexTask(task *IndexTask, wg *sync.WaitGroup) {
	task.Wg = wg // Assign WaitGroup to task

	// Apply debouncing for file updates (not for manual rebuilds)
	if task.Priority < 10 { // Priority 10 is for manual rebuilds, should not be debounced
		li.debounceIndexTask(task)
	} else {
		// Manual rebuilds bypass debouncing
		li.executeIndexTask(task)
	}
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
				}, nil) // No waitgroup for autodetected compressed file
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
					}, nil) // No waitgroup for file updates
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
