package nginx_log

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/uozi-tech/cosy/logger"
)

// TaskRecovery handles the recovery of incomplete indexing tasks after restart
type TaskRecovery struct {
	logFileManager *indexer.LogFileManager
	modernIndexer  *indexer.ParallelIndexer
	activeTasks    int32 // Counter for active recovery tasks
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// NewTaskRecovery creates a new task recovery manager
func NewTaskRecovery(parentCtx context.Context) *TaskRecovery {
	ctx, cancel := context.WithCancel(parentCtx)
	return &TaskRecovery{
		logFileManager: GetLogFileManager(),
		modernIndexer:  GetModernIndexer(),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// RecoverUnfinishedTasks recovers indexing tasks that were incomplete at last shutdown
func (tr *TaskRecovery) RecoverUnfinishedTasks(ctx context.Context) error {
	if tr.logFileManager == nil || tr.modernIndexer == nil {
		logger.Warn("Cannot recover tasks: services not available")
		return nil
	}

	logger.Debug("Starting recovery of unfinished indexing tasks")

	// Get all logs with their index status
	allLogs := GetAllLogsWithIndexGrouped(func(log *NginxLogWithIndex) bool {
		// Only process access logs
		return log.Type == "access"
	})

	var incompleteTasksCount int
	var queuePosition int = 1

	for _, log := range allLogs {
		if tr.needsRecovery(log) {
			incompleteTasksCount++

			// Reset to queued status and assign queue position
			if err := tr.recoverTask(ctx, log.Path, queuePosition); err != nil {
				logger.Errorf("Failed to recover task for %s: %v", log.Path, err)
			} else {
				queuePosition++
			}
		}
	}

	if incompleteTasksCount > 0 {
		logger.Debugf("Recovered %d incomplete indexing tasks", incompleteTasksCount)
	} else {
		logger.Debug("No incomplete indexing tasks found")
	}

	return nil
}

// needsRecovery determines if a log file has an incomplete indexing task that needs recovery
func (tr *TaskRecovery) needsRecovery(log *NginxLogWithIndex) bool {
	// Check for incomplete states that indicate interrupted operations
	switch log.IndexStatus {
	case string(indexer.IndexStatusIndexing):
		// Task was in progress during last shutdown
		logger.Debugf("Found incomplete indexing task: %s", log.Path)
		return true

	case string(indexer.IndexStatusQueued):
		// Task was queued but may not have started
		logger.Debugf("Found queued indexing task: %s", log.Path)
		return true

	case string(indexer.IndexStatusError):
		// Check if error is recent (within last hour before restart)
		if log.LastIndexed > 0 {
			lastIndexTime := time.Unix(log.LastIndexed, 0)
			if time.Since(lastIndexTime) < time.Hour {
				logger.Debugf("Found recent error task for retry: %s", log.Path)
				return true
			}
		}
	}

	return false
}

// recoverTask recovers a single indexing task
func (tr *TaskRecovery) recoverTask(ctx context.Context, logPath string, queuePosition int) error {
	logger.Debugf("Recovering indexing task for: %s (queue position: %d)", logPath, queuePosition)

	// Set status to queued with queue position
	if err := tr.setTaskStatus(logPath, string(indexer.IndexStatusQueued), queuePosition); err != nil {
		return err
	}

	// Queue the recovery task asynchronously with proper context and WaitGroup
	tr.wg.Add(1)
	go tr.executeRecoveredTask(tr.ctx, logPath)

	return nil
}

// executeRecoveredTask executes a recovered indexing task with proper global state and progress tracking
func (tr *TaskRecovery) executeRecoveredTask(ctx context.Context, logPath string) {
	defer tr.wg.Done() // Always decrement WaitGroup

	// Check context before starting
	select {
	case <-ctx.Done():
		logger.Debugf("Context cancelled, skipping recovery task for %s", logPath)
		return
	default:
	}

	// Add a small delay to stagger recovery tasks, but check context
	select {
	case <-time.After(time.Second * 2):
	case <-ctx.Done():
		logger.Debugf("Context cancelled during delay, skipping recovery task for %s", logPath)
		return
	}

	logger.Debugf("Executing recovered indexing task: %s", logPath)

	// Get processing manager for global state updates
	processingManager := event.GetProcessingStatusManager()

	// Increment active tasks counter and set global status if this is the first task
	isFirstTask := atomic.AddInt32(&tr.activeTasks, 1) == 1
	if isFirstTask {
		processingManager.UpdateNginxLogIndexing(true)
		logger.Debug("Set global indexing status to true for recovery tasks")
	}

	// Ensure we always decrement counter and reset global status when no tasks remain
	defer func() {
		remainingTasks := atomic.AddInt32(&tr.activeTasks, -1)
		if remainingTasks == 0 {
			processingManager.UpdateNginxLogIndexing(false)
			logger.Debug("Set global indexing status to false - all recovery tasks completed")
		}
		if r := recover(); r != nil {
			logger.Errorf("Panic during recovered task execution: %v", r)
		}
	}()

	// Set status to indexing
	if err := tr.setTaskStatus(logPath, string(indexer.IndexStatusIndexing), 0); err != nil {
		logger.Errorf("Failed to set indexing status for recovered task %s: %v", logPath, err)
		return
	}

	// Create progress tracking configuration for recovery task
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

			logger.Debugf("Recovery progress: %s - %.1f%% (Files: %d/%d, Lines: %d/%d)",
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

			logger.Debugf("Recovery completion: %s - Success: %t, Duration: %s, Lines: %d, Size: %d bytes",
				completion.LogGroupPath, completion.Success, completion.Duration,
				completion.TotalLines, completion.IndexedSize)

			// Send index ready event if recovery was successful
			if completion.Success {
				event.Publish(event.Event{
					Type: event.TypeNginxLogIndexReady,
					Data: event.NginxLogIndexReadyData{
						LogPath:     completion.LogGroupPath,
						StartTime:   time.Now().Unix(),
						EndTime:     time.Now().Unix(),
						Available:   true,
						IndexStatus: "ready",
					},
				})
			}
		},
	}

	// Check context before starting indexing
	select {
	case <-ctx.Done():
		logger.Debugf("Context cancelled before indexing, stopping recovery task for %s", logPath)
		return
	default:
	}

	// Execute the indexing with progress tracking
	startTime := time.Now()
	docsCountMap, minTime, maxTime, err := tr.modernIndexer.IndexLogGroupWithProgress(logPath, progressConfig)

	if err != nil {
		logger.Errorf("Failed to execute recovered indexing task %s: %v", logPath, err)
		// Set error status
		if statusErr := tr.setTaskStatus(logPath, string(indexer.IndexStatusError), 0); statusErr != nil {
			logger.Errorf("Failed to set error status for recovered task %s: %v", logPath, statusErr)
		}
		return
	}

	// Calculate total documents indexed
	var totalDocsIndexed uint64
	for _, docCount := range docsCountMap {
		totalDocsIndexed += docCount
	}

	// Save indexing metadata using the log file manager
	duration := time.Since(startTime)
	if err := tr.logFileManager.SaveIndexMetadata(logPath, totalDocsIndexed, startTime, duration, minTime, maxTime); err != nil {
		logger.Errorf("Failed to save recovered index metadata for %s: %v", logPath, err)
	}

	// Set status to indexed (completed)
	if err := tr.setTaskStatus(logPath, string(indexer.IndexStatusIndexed), 0); err != nil {
		logger.Errorf("Failed to set completed status for recovered task %s: %v", logPath, err)
	}

	// Update searcher shards
	UpdateSearcherShards()

	logger.Debugf("Successfully completed recovered indexing task: %s, Documents: %d", logPath, totalDocsIndexed)
}

// setTaskStatus updates the task status in the database using the enhanced persistence layer
func (tr *TaskRecovery) setTaskStatus(logPath, status string, queuePosition int) error {
	// Get persistence manager
	persistence := tr.logFileManager.GetPersistence()
	if persistence == nil {
		return fmt.Errorf("persistence manager not available")
	}

	// Use enhanced SetIndexStatus method
	return persistence.SetIndexStatus(logPath, status, queuePosition, "")
}

// Shutdown gracefully stops all recovery tasks
func (tr *TaskRecovery) Shutdown() {
	logger.Debug("Shutting down task recovery system...")

	// Cancel all active tasks
	tr.cancel()

	// Wait for all tasks to complete with timeout
	done := make(chan struct{})
	go func() {
		tr.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Debug("All recovery tasks completed successfully")
	case <-time.After(30 * time.Second):
		logger.Warn("Timeout waiting for recovery tasks to complete")
	}

	logger.Debug("Task recovery system shutdown completed")
}

// Global task recovery manager
var globalTaskRecovery *TaskRecovery

// InitTaskRecovery initializes the task recovery system - called during application startup
func InitTaskRecovery(ctx context.Context) {
	logger.Debug("Initializing task recovery system")

	// Wait a bit for services to fully initialize
	time.Sleep(3 * time.Second)

	globalTaskRecovery = NewTaskRecovery(ctx)
	if err := globalTaskRecovery.RecoverUnfinishedTasks(ctx); err != nil {
		logger.Errorf("Failed to recover unfinished tasks: %v", err)
	}

	// Monitor context for shutdown
	go func() {
		<-ctx.Done()
		if globalTaskRecovery != nil {
			globalTaskRecovery.Shutdown()
		}
	}()
}
