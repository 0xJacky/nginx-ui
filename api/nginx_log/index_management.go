package nginx_log

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// rebuildLocks tracks ongoing rebuild operations for specific log groups
var (
	rebuildLocks     = make(map[string]*sync.Mutex)
	rebuildLocksLock sync.RWMutex
)

// acquireRebuildLock gets or creates a mutex for a specific log group
func acquireRebuildLock(logGroupPath string) *sync.Mutex {
	rebuildLocksLock.Lock()
	defer rebuildLocksLock.Unlock()
	
	if lock, exists := rebuildLocks[logGroupPath]; exists {
		return lock
	}
	
	lock := &sync.Mutex{}
	rebuildLocks[logGroupPath] = lock
	return lock
}

// releaseRebuildLock removes the mutex for a specific log group after completion
func releaseRebuildLock(logGroupPath string) {
	rebuildLocksLock.Lock()
	defer rebuildLocksLock.Unlock()
	delete(rebuildLocks, logGroupPath)
}

// isRebuildInProgress checks if a rebuild is currently running for a specific log group
func isRebuildInProgress(logGroupPath string) bool {
	rebuildLocksLock.RLock()
	defer rebuildLocksLock.RUnlock()
	
	if lock, exists := rebuildLocks[logGroupPath]; exists {
		// Try to acquire the lock with a short timeout
		// If we can't acquire it, it means rebuild is in progress
		if lock.TryLock() {
			lock.Unlock()
			return false
		}
		return true
	}
	return false
}

// RebuildIndex rebuilds the log index asynchronously (all files or specific file)
// The API call returns immediately and the rebuild happens in background
func RebuildIndex(c *gin.Context) {
	var request controlStruct
	if err := c.ShouldBindJSON(&request); err != nil {
		// No JSON body means rebuild all indexes
		request.Path = ""
	}

	// Get modern indexer
	modernIndexer := nginx_log.GetIndexer()
	if modernIndexer == nil {
		cosy.ErrHandler(c, nginx_log.ErrModernIndexerNotAvailable)
		return
	}

	// Check if modern indexer is healthy
	if !modernIndexer.IsHealthy() {
		cosy.ErrHandler(c, fmt.Errorf("modern indexer is not healthy"))
		return
	}

	// Check if indexing is already in progress
	processingManager := event.GetProcessingStatusManager()
	currentStatus := processingManager.GetCurrentStatus()
	if currentStatus.NginxLogIndexing {
		cosy.ErrHandler(c, nginx_log.ErrFailedToRebuildIndex)
		return
	}

	// Check if specific log group rebuild is already in progress using task scheduler
	scheduler := nginx_log.GetTaskScheduler()
	if request.Path != "" {
		if scheduler != nil && scheduler.IsTaskInProgress(request.Path) {
			cosy.ErrHandler(c, nginx_log.ErrFailedToRebuildFileIndex)
			return
		}
		// Fallback to local lock check if scheduler not available
		if scheduler == nil && isRebuildInProgress(request.Path) {
			cosy.ErrHandler(c, nginx_log.ErrFailedToRebuildFileIndex)
			return
		}
	}

	// Return immediate response to client
	c.JSON(http.StatusOK, IndexRebuildResponse{
		Message: "Index rebuild started in background",
		Status:  "started",
	})

	// Start async rebuild in goroutine
	go func() {
		performAsyncRebuild(modernIndexer, request.Path)
	}()
}

// performAsyncRebuild performs the actual rebuild logic asynchronously
// For incremental indexing of a specific log group, it preserves existing metadata
// For full rebuilds (path == ""), it clears all metadata first
func performAsyncRebuild(modernIndexer interface{}, path string) {
	processingManager := event.GetProcessingStatusManager()

	// Notify that indexing has started
	processingManager.UpdateNginxLogIndexing(true)

	// Create a context for this rebuild operation that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ensure we always reset status when done
	defer func() {
		processingManager.UpdateNginxLogIndexing(false)
		if r := recover(); r != nil {
			logger.Errorf("Panic during async rebuild: %v", r)
		}
	}()

	logFileManager := nginx_log.GetLogFileManager()

	// Handle index cleanup based on rebuild scope
	if path != "" {
		// For single file rebuild, only delete indexes for that log group
		if err := modernIndexer.(*indexer.ParallelIndexer).DeleteIndexByLogGroup(path, logFileManager); err != nil {
			logger.Errorf("Failed to delete existing indexes for log group %s: %v", path, err)
			return
		}
		logger.Infof("Deleted existing indexes for log group: %s", path)
	} else {
		// For full rebuild, destroy all existing indexes with context
		if err := nginx_log.DestroyAllIndexes(ctx); err != nil {
			logger.Errorf("Failed to destroy existing indexes before rebuild: %v", err)
			return
		}

		// Re-initialize the indexer to create new, empty shards
		if err := modernIndexer.(indexer.RestartableIndexer).Start(ctx); err != nil {
			logger.Errorf("Failed to re-initialize indexer after destruction: %v", err)
			return
		}
		logger.Info("Re-initialized indexer after destruction")
	}

	// Create progress tracking callbacks
	progressConfig := &indexer.ProgressConfig{
		NotifyInterval: 1 * time.Second,
		OnProgress: func(progress indexer.ProgressNotification) {
			// Send progress event to frontend
			event.Publish(event.Event{
				Type: event.TypeNginxLogIndexProgress,
				Data: event.NginxLogIndexProgressData{
					LogPath:         progress.LogGroupPath,
					Progress:        progress.Percentage,
					Stage:           "indexing",
					Status:          "running",
					ElapsedTime:     progress.ElapsedTime.Milliseconds(),
					EstimatedRemain: progress.EstimatedRemain.Milliseconds(),
				},
			})

			logger.Infof("Index progress: %s - %.1f%% (Files: %d/%d, Lines: %d/%d)",
				progress.LogGroupPath, progress.Percentage, progress.CompletedFiles,
				progress.TotalFiles, progress.ProcessedLines, progress.EstimatedLines)
		},
		OnCompletion: func(completion indexer.CompletionNotification) {
			// Send completion event to frontend
			event.Publish(event.Event{
				Type: event.TypeNginxLogIndexComplete,
				Data: event.NginxLogIndexCompleteData{
					LogPath:     completion.LogGroupPath,
					Success:     completion.Success,
					Duration:    int64(completion.Duration.Milliseconds()),
					TotalLines:  completion.TotalLines,
					IndexedSize: completion.IndexedSize,
					Error:       completion.Error,
				},
			})

			logger.Infof("Index completion: %s - Success: %t, Duration: %s, Lines: %d, Size: %d bytes",
				completion.LogGroupPath, completion.Success, completion.Duration,
				completion.TotalLines, completion.IndexedSize)
		},
	}

	// Store the progress config to access from rebuild functions
	var globalMinTime, globalMaxTime *time.Time

	// Create a wrapper progress config that captures timing information
	wrapperProgressConfig := &indexer.ProgressConfig{
		NotifyInterval: progressConfig.NotifyInterval,
		OnProgress:     progressConfig.OnProgress,
		OnCompletion: func(completion indexer.CompletionNotification) {
			// Call the original completion callback first
			if progressConfig.OnCompletion != nil {
				progressConfig.OnCompletion(completion)
			}

			// Send index ready event if indexing was successful with actual time range
			if completion.Success {
				var startTimeUnix, endTimeUnix int64

				// Use global timing if available, otherwise use current time
				if globalMinTime != nil {
					startTimeUnix = globalMinTime.Unix()
				} else {
					startTimeUnix = time.Now().Unix()
				}

				if globalMaxTime != nil {
					endTimeUnix = globalMaxTime.Unix()
				} else {
					endTimeUnix = time.Now().Unix()
				}

				event.Publish(event.Event{
					Type: event.TypeNginxLogIndexReady,
					Data: event.NginxLogIndexReadyData{
						LogPath:     completion.LogGroupPath,
						StartTime:   startTimeUnix,
						EndTime:     endTimeUnix,
						Available:   true,
						IndexStatus: "ready",
					},
				})
			}
		},
	}

	if path != "" {
		// Rebuild specific file
		minTime, maxTime := rebuildSingleFile(modernIndexer, path, logFileManager, wrapperProgressConfig)
		globalMinTime, globalMaxTime = minTime, maxTime
	} else {
		// Rebuild all indexes
		minTime, maxTime := rebuildAllFiles(modernIndexer, logFileManager, wrapperProgressConfig)
		globalMinTime, globalMaxTime = minTime, maxTime
	}
}

// rebuildSingleFile rebuilds index for a single file
func rebuildSingleFile(modernIndexer interface{}, path string, logFileManager interface{}, progressConfig *indexer.ProgressConfig) (*time.Time, *time.Time) {
	// Use task scheduler lock if available for unified locking across recovery and manual rebuild
	scheduler := nginx_log.GetTaskScheduler()

	var unlock func()
	if scheduler != nil {
		// Use scheduler's unified lock
		_, unlock = scheduler.AcquireTaskLock(path)
		defer unlock()
	} else {
		// Fallback: Acquire local lock for this specific log group
		lock := acquireRebuildLock(path)
		lock.Lock()
		defer func() {
			lock.Unlock()
			releaseRebuildLock(path)
		}()
	}
	// For a single file, we need to check its type first
	allLogsForTypeCheck := nginx_log.GetAllLogsWithIndexGrouped()
	var targetLog *nginx_log.NginxLogWithIndex
	for _, log := range allLogsForTypeCheck {
		if log.Path == path {
			targetLog = log
			break
		}
	}

	var minTime, maxTime *time.Time

	if targetLog != nil && targetLog.Type == "error" {
		logger.Infof("Skipping index rebuild for error log as requested: %s", path)
		if logFileManager != nil {
			if err := logFileManager.(indexer.MetadataManager).SaveIndexMetadata(path, 0, time.Now(), 0, nil, nil); err != nil {
				logger.Warnf("Could not reset metadata for skipped error log %s: %v", path, err)
			}
		}
	} else {
		logger.Infof("Starting modern index rebuild for file: %s", path)

		// NOTE: We intentionally do NOT delete existing index metadata here
		// This allows incremental indexing to work properly with rotated logs
		// The IndexLogGroupWithProgress method will handle updating existing records

		startTime := time.Now()

		docsCountMap, docMinTime, docMaxTime, err := modernIndexer.(*indexer.ParallelIndexer).IndexLogGroupWithProgress(path, progressConfig)

		if err != nil {
			logger.Errorf("Failed to index modern index for file group %s: %v", path, err)
			return nil, nil
		}

		minTime, maxTime = docMinTime, docMaxTime

		duration := time.Since(startTime)
		var totalDocsIndexed uint64
		if logFileManager != nil {
			// Calculate total document count
			for _, docCount := range docsCountMap {
				totalDocsIndexed += docCount
			}
			
			// Save metadata for the base log path with total count
			if err := logFileManager.(indexer.MetadataManager).SaveIndexMetadata(path, totalDocsIndexed, startTime, duration, minTime, maxTime); err != nil {
				logger.Errorf("Failed to save index metadata for %s: %v", path, err)
			}
			
			// Also save individual file metadata if needed
			for filePath, docCount := range docsCountMap {
				if filePath != path { // Don't duplicate the base path
					if err := logFileManager.(indexer.MetadataManager).SaveIndexMetadata(filePath, docCount, startTime, duration, minTime, maxTime); err != nil {
						logger.Errorf("Failed to save index metadata for %s: %v", filePath, err)
					}
				}
			}
		}
		logger.Infof("Successfully completed modern rebuild for file group: %s, Documents: %d", path, totalDocsIndexed)
	}

	if err := modernIndexer.(indexer.FlushableIndexer).FlushAll(); err != nil {
		logger.Errorf("Failed to flush all indexer data for single file: %v", err)
	}
	nginx_log.UpdateSearcherShards()

	return minTime, maxTime
}

// rebuildAllFiles rebuilds indexes for all files with proper queue management
func rebuildAllFiles(modernIndexer interface{}, logFileManager interface{}, progressConfig *indexer.ProgressConfig) (*time.Time, *time.Time) {
	// For full rebuild, we use a special global lock key
	globalLockKey := "__GLOBAL_REBUILD__"

	// Use task scheduler lock if available for unified locking
	scheduler := nginx_log.GetTaskScheduler()
	var unlock func()
	if scheduler != nil {
		// Use scheduler's unified lock
		_, unlock = scheduler.AcquireTaskLock(globalLockKey)
		defer unlock()
	} else {
		// Fallback: Acquire local lock
		lock := acquireRebuildLock(globalLockKey)
		lock.Lock()
		defer func() {
			lock.Unlock()
			releaseRebuildLock(globalLockKey)
		}()
	}
	
	// For full rebuild, we clear ALL existing metadata to start fresh
	// This is different from single file/group rebuild which preserves metadata for incremental indexing
	if logFileManager != nil {
		if err := logFileManager.(indexer.MetadataManager).DeleteAllIndexMetadata(); err != nil {
			logger.Errorf("Could not clean up all old DB records before full rebuild: %v", err)
		}
	}

	logger.Info("Starting full modern index rebuild with queue management")
	allLogs := nginx_log.GetAllLogsWithIndexGrouped()
	
	// Get persistence manager for queue management
	var persistence *indexer.PersistenceManager
	if lfm, ok := logFileManager.(*indexer.LogFileManager); ok {
		persistence = lfm.GetPersistence()
	}

	// First pass: Set all access logs to queued status
	queuePosition := 1
	accessLogs := make([]*nginx_log.NginxLogWithIndex, 0)
	
	for _, log := range allLogs {
		if log.Type == "error" {
			logger.Infof("Skipping indexing for error log: %s", log.Path)
			if logFileManager != nil {
				if err := logFileManager.(indexer.MetadataManager).SaveIndexMetadata(log.Path, 0, time.Now(), 0, nil, nil); err != nil {
					logger.Warnf("Could not reset metadata for skipped error log %s: %v", log.Path, err)
				}
			}
			continue
		}
		
		// Set to queued status with position
		if persistence != nil {
			if err := persistence.SetIndexStatus(log.Path, string(indexer.IndexStatusQueued), queuePosition, ""); err != nil {
				logger.Errorf("Failed to set queued status for %s: %v", log.Path, err)
			}
		}
		
		accessLogs = append(accessLogs, log)
		queuePosition++
	}

	// Give the frontend a moment to refresh and show queued status
	time.Sleep(2 * time.Second)

	startTime := time.Now()
	var overallMinTime, overallMaxTime *time.Time
	var timeMu sync.Mutex

	// Second pass: Process queued logs in parallel with controlled concurrency
	var wg sync.WaitGroup
	// Get concurrency from indexer config (FileGroupConcurrency controls both file and group level parallelism)
	maxConcurrency := 4 // Default fallback
	if pi, ok := modernIndexer.(*indexer.ParallelIndexer); ok {
		config := pi.GetConfig()
		if config.FileGroupConcurrency > 0 {
			maxConcurrency = config.FileGroupConcurrency
		}
	}
	semaphore := make(chan struct{}, maxConcurrency)

	logger.Infof("Processing %d log groups in parallel with concurrency=%d", len(accessLogs), maxConcurrency)

	for _, log := range accessLogs {
		wg.Add(1)
		go func(logItem *nginx_log.NginxLogWithIndex) {
			defer wg.Done()

			// Acquire semaphore for controlled concurrency
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Set to indexing status
			if persistence != nil {
				if err := persistence.SetIndexStatus(logItem.Path, string(indexer.IndexStatusIndexing), 0, ""); err != nil {
					logger.Errorf("Failed to set indexing status for %s: %v", logItem.Path, err)
				}
			}

			loopStartTime := time.Now()
			docsCountMap, minTime, maxTime, err := modernIndexer.(*indexer.ParallelIndexer).IndexLogGroupWithProgress(logItem.Path, progressConfig)

			if err != nil {
				logger.Warnf("Failed to index file group: %s, error: %v", logItem.Path, err)
				// Set error status
				if persistence != nil {
					if err := persistence.SetIndexStatus(logItem.Path, string(indexer.IndexStatusError), 0, err.Error()); err != nil {
						logger.Errorf("Failed to set error status for %s: %v", logItem.Path, err)
					}
				}
			} else {
				// Track overall time range across all log files (thread-safe)
				timeMu.Lock()
				if minTime != nil {
					if overallMinTime == nil || minTime.Before(*overallMinTime) {
						overallMinTime = minTime
					}
				}
				if maxTime != nil {
					if overallMaxTime == nil || maxTime.After(*overallMaxTime) {
						overallMaxTime = maxTime
					}
				}
				timeMu.Unlock()

				if logFileManager != nil {
					duration := time.Since(loopStartTime)
					// Calculate total document count for the log group
					var totalDocCount uint64
					for _, docCount := range docsCountMap {
						totalDocCount += docCount
					}

					// Save metadata for the base log path with total count
					if err := logFileManager.(indexer.MetadataManager).SaveIndexMetadata(logItem.Path, totalDocCount, loopStartTime, duration, minTime, maxTime); err != nil {
						logger.Errorf("Failed to save index metadata for %s: %v", logItem.Path, err)
					}

					// Also save individual file metadata if needed
					for path, docCount := range docsCountMap {
						if path != logItem.Path { // Don't duplicate the base path
							if err := logFileManager.(indexer.MetadataManager).SaveIndexMetadata(path, docCount, loopStartTime, duration, minTime, maxTime); err != nil {
								logger.Errorf("Failed to save index metadata for %s: %v", path, err)
							}
						}
					}
				}

				// Set to indexed status
				if persistence != nil {
					if err := persistence.SetIndexStatus(logItem.Path, string(indexer.IndexStatusIndexed), 0, ""); err != nil {
						logger.Errorf("Failed to set indexed status for %s: %v", logItem.Path, err)
					}
				}
			}
		}(log)
	}

	// Wait for all log groups to complete
	wg.Wait()

	totalDuration := time.Since(startTime)
	logger.Infof("Successfully completed full modern index rebuild in %s", totalDuration)

	if err := modernIndexer.(indexer.FlushableIndexer).FlushAll(); err != nil {
		logger.Errorf("Failed to flush all indexer data: %v", err)
	}

	nginx_log.UpdateSearcherShards()

	return overallMinTime, overallMaxTime
}
