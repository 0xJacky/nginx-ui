package nginx_log

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// RebuildIndex rebuilds the log index asynchronously (all files or specific file)
// The API call returns immediately and the rebuild happens in background
func RebuildIndex(c *gin.Context) {
	var request controlStruct
	if err := c.ShouldBindJSON(&request); err != nil {
		// No JSON body means rebuild all indexes
		request.Path = ""
	}

	// Get modern indexer
	modernIndexer := nginx_log.GetModernIndexer()
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
		cosy.ErrHandler(c, fmt.Errorf("index rebuild is already in progress"))
		return
	}

	// Return immediate response to client
	c.JSON(http.StatusOK, gin.H{
		"message": "Index rebuild started in background",
		"status":  "started",
	})

	// Start async rebuild in goroutine
	go func() {
		performAsyncRebuild(modernIndexer, request.Path)
	}()
}

// performAsyncRebuild performs the actual rebuild logic asynchronously
func performAsyncRebuild(modernIndexer interface{}, path string) {
	processingManager := event.GetProcessingStatusManager()
	
	// Notify that indexing has started
	processingManager.UpdateNginxLogIndexing(true)
	
	// Ensure we always reset status when done
	defer func() {
		processingManager.UpdateNginxLogIndexing(false)
		if r := recover(); r != nil {
			logger.Errorf("Panic during async rebuild: %v", r)
		}
	}()

	// First, destroy all existing indexes to ensure a clean slate
	if err := nginx_log.DestroyAllIndexes(); err != nil {
		logger.Errorf("Failed to destroy existing indexes before rebuild: %v", err)
		return
	}

	// Re-initialize the indexer to create new, empty shards
	if err := modernIndexer.(interface {
		Start(context.Context) error
	}).Start(context.Background()); err != nil {
		logger.Errorf("Failed to re-initialize indexer after destruction: %v", err)
		return
	}

	logFileManager := nginx_log.GetLogFileManager()

	// Create progress tracking callbacks
	progressConfig := &indexer.ProgressConfig{
		NotifyInterval: 2 * time.Second,
		OnProgress: func(progress indexer.ProgressNotification) {
			// Send progress event to frontend
			event.Publish(event.Event{
				Type: event.EventTypeNginxLogIndexProgress,
				Data: event.NginxLogIndexProgressData{
					LogPath:         progress.LogGroupPath,
					Progress:        progress.Percentage,
					Stage:           "indexing",
					Status:          "running",
					ElapsedTime:     int64(progress.ElapsedTime.Milliseconds()),
					EstimatedRemain: int64(progress.EstimatedRemain.Milliseconds()),
				},
			})
			
			logger.Infof("Index progress: %s - %.1f%% (Files: %d/%d, Lines: %d/%d)", 
				progress.LogGroupPath, progress.Percentage, progress.CompletedFiles, 
				progress.TotalFiles, progress.ProcessedLines, progress.EstimatedLines)
		},
		OnCompletion: func(completion indexer.CompletionNotification) {
			// Send completion event to frontend
			event.Publish(event.Event{
				Type: event.EventTypeNginxLogIndexComplete,
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
					Type: event.EventTypeNginxLogIndexReady,
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
			if err := logFileManager.(interface {
				SaveIndexMetadata(string, uint64, time.Time, time.Duration, *time.Time, *time.Time) error
			}).SaveIndexMetadata(path, 0, time.Now(), 0, nil, nil); err != nil {
				logger.Warnf("Could not reset metadata for skipped error log %s: %v", path, err)
			}
		}
	} else {
		logger.Infof("Starting modern index rebuild for file: %s", path)
		
		// Clear existing database records for this log group before rebuilding
		if logFileManager != nil {
			if err := logFileManager.(interface {
				DeleteIndexMetadataByGroup(string) error
			}).DeleteIndexMetadataByGroup(path); err != nil {
				logger.Warnf("Could not clean up existing DB records for log group %s: %v", path, err)
			}
		}
		
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
			for filePath, docCount := range docsCountMap {
				totalDocsIndexed += docCount
				if err := logFileManager.(interface {
					SaveIndexMetadata(string, uint64, time.Time, time.Duration, *time.Time, *time.Time) error
				}).SaveIndexMetadata(filePath, docCount, startTime, duration, minTime, maxTime); err != nil {
					logger.Errorf("Failed to save index metadata for %s: %v", filePath, err)
				}
			}
		}
		logger.Infof("Successfully completed modern rebuild for file group: %s, Documents: %d", path, totalDocsIndexed)
	}

	if err := modernIndexer.(interface {
		FlushAll() error
	}).FlushAll(); err != nil {
		logger.Errorf("Failed to flush all indexer data for single file: %v", err)
	}
	nginx_log.UpdateSearcherShards()
	
	return minTime, maxTime
}

// rebuildAllFiles rebuilds indexes for all files
func rebuildAllFiles(modernIndexer interface{}, logFileManager interface{}, progressConfig *indexer.ProgressConfig) (*time.Time, *time.Time) {
	if logFileManager != nil {
		if err := logFileManager.(interface {
			DeleteAllIndexMetadata() error
		}).DeleteAllIndexMetadata(); err != nil {
			logger.Errorf("Could not clean up all old DB records before full rebuild: %v", err)
		}
	}

	logger.Info("Starting full modern index rebuild")
	allLogs := nginx_log.GetAllLogsWithIndexGrouped()

	startTime := time.Now()
	var overallMinTime, overallMaxTime *time.Time

	for _, log := range allLogs {
		if log.Type == "error" {
			logger.Infof("Skipping indexing for error log: %s", log.Path)
			if logFileManager != nil {
				if err := logFileManager.(interface {
					SaveIndexMetadata(string, uint64, time.Time, time.Duration, *time.Time, *time.Time) error
				}).SaveIndexMetadata(log.Path, 0, time.Now(), 0, nil, nil); err != nil {
					logger.Warnf("Could not reset metadata for skipped error log %s: %v", log.Path, err)
				}
			}
			continue
		}

		loopStartTime := time.Now()
		docsCountMap, minTime, maxTime, err := modernIndexer.(*indexer.ParallelIndexer).IndexLogGroupWithProgress(log.Path, progressConfig)
		
		if err != nil {
			logger.Warnf("Failed to index file group, skipping: %s, error: %v", log.Path, err)
		} else {
			// Track overall time range across all log files
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
			
			if logFileManager != nil {
				duration := time.Since(loopStartTime)
				for path, docCount := range docsCountMap {
					if err := logFileManager.(interface {
						SaveIndexMetadata(string, uint64, time.Time, time.Duration, *time.Time, *time.Time) error
					}).SaveIndexMetadata(path, docCount, loopStartTime, duration, minTime, maxTime); err != nil {
						logger.Errorf("Failed to save index metadata for %s: %v", path, err)
					}
				}
			}
		}
	}

	totalDuration := time.Since(startTime)
	logger.Infof("Successfully completed full modern index rebuild in %s", totalDuration)

	if err := modernIndexer.(interface {
		FlushAll() error
	}).FlushAll(); err != nil {
		logger.Errorf("Failed to flush all indexer data: %v", err)
	}

	nginx_log.UpdateSearcherShards()
	
	return overallMinTime, overallMaxTime
}
