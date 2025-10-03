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

// TaskScheduler manages all indexing tasks (recovery, manual rebuild, etc.)
// with unified locking to prevent concurrent execution on the same log group
type TaskScheduler struct {
	logFileManager *indexer.LogFileManager
	modernIndexer  *indexer.ParallelIndexer
	activeTasks    int32              // Counter for active tasks
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	taskLocks      map[string]*sync.Mutex // Per-log-group locks
	locksMutex     sync.RWMutex           // Protects taskLocks map
}

// Global task scheduler instance
var (
	globalTaskScheduler     *TaskScheduler
	taskSchedulerOnce       sync.Once
	taskSchedulerInitialized bool
	taskSchedulerMutex      sync.RWMutex
)

// GetTaskScheduler returns the global task scheduler instance
func GetTaskScheduler() *TaskScheduler {
	taskSchedulerMutex.RLock()
	defer taskSchedulerMutex.RUnlock()
	return globalTaskScheduler
}

// InitTaskScheduler initializes the global task scheduler
func InitTaskScheduler(ctx context.Context) {
	taskSchedulerMutex.Lock()
	defer taskSchedulerMutex.Unlock()

	if taskSchedulerInitialized {
		logger.Debug("Task scheduler already initialized")
		return
	}

	logger.Debug("Initializing task scheduler")

	// Wait a bit for services to fully initialize
	time.Sleep(3 * time.Second)

	// Check if services are available
	if GetLogFileManager() == nil || GetIndexer() == nil {
		logger.Debug("Modern services not available, skipping task scheduler initialization")
		return
	}

	globalTaskScheduler = NewTaskScheduler(ctx)
	taskSchedulerInitialized = true

	// Start task recovery
	if err := globalTaskScheduler.RecoverUnfinishedTasks(ctx); err != nil {
		logger.Errorf("Failed to recover unfinished tasks: %v", err)
	}

	// Monitor context for shutdown
	go func() {
		<-ctx.Done()
		if globalTaskScheduler != nil {
			globalTaskScheduler.Shutdown()
		}
	}()
}

// NewTaskScheduler creates a new task scheduler
func NewTaskScheduler(parentCtx context.Context) *TaskScheduler {
	ctx, cancel := context.WithCancel(parentCtx)
	return &TaskScheduler{
		logFileManager: GetLogFileManager(),
		modernIndexer:  GetIndexer(),
		ctx:            ctx,
		cancel:         cancel,
		taskLocks:      make(map[string]*sync.Mutex),
	}
}

// acquireTaskLock gets or creates a mutex for a specific log group
func (ts *TaskScheduler) acquireTaskLock(logPath string) *sync.Mutex {
	ts.locksMutex.Lock()
	defer ts.locksMutex.Unlock()

	if lock, exists := ts.taskLocks[logPath]; exists {
		return lock
	}

	lock := &sync.Mutex{}
	ts.taskLocks[logPath] = lock
	return lock
}

// releaseTaskLock removes the mutex for a specific log group after completion
func (ts *TaskScheduler) releaseTaskLock(logPath string) {
	ts.locksMutex.Lock()
	defer ts.locksMutex.Unlock()
	delete(ts.taskLocks, logPath)
}

// IsTaskInProgress checks if a task is currently running for a specific log group
func (ts *TaskScheduler) IsTaskInProgress(logPath string) bool {
	ts.locksMutex.RLock()
	defer ts.locksMutex.RUnlock()

	if lock, exists := ts.taskLocks[logPath]; exists {
		// Try to acquire the lock with TryLock
		// If we can't acquire it, it means task is in progress
		if lock.TryLock() {
			lock.Unlock()
			return false
		}
		return true
	}
	return false
}

// AcquireTaskLock acquires a lock for external use (e.g., manual rebuild)
// Returns the lock and a release function
func (ts *TaskScheduler) AcquireTaskLock(logPath string) (*sync.Mutex, func()) {
	lock := ts.acquireTaskLock(logPath)
	lock.Lock()

	releaseFunc := func() {
		lock.Unlock()
		ts.releaseTaskLock(logPath)
	}

	return lock, releaseFunc
}

// ScheduleIndexTask schedules an indexing task for a log group
// Returns error if task is already in progress
func (ts *TaskScheduler) ScheduleIndexTask(ctx context.Context, logPath string, progressConfig *indexer.ProgressConfig) error {
	// Check if task is already in progress
	if ts.IsTaskInProgress(logPath) {
		return fmt.Errorf("indexing task already in progress for %s", logPath)
	}

	// Queue the task asynchronously with proper context and WaitGroup
	ts.wg.Add(1)
	go ts.executeIndexTask(ctx, logPath, progressConfig)

	return nil
}

// executeIndexTask executes an indexing task with proper locking and progress tracking
func (ts *TaskScheduler) executeIndexTask(ctx context.Context, logPath string, progressConfig *indexer.ProgressConfig) {
	defer ts.wg.Done() // Always decrement WaitGroup

	// Acquire lock for this specific log group to prevent concurrent execution
	lock := ts.acquireTaskLock(logPath)
	lock.Lock()
	defer func() {
		lock.Unlock()
		ts.releaseTaskLock(logPath)
	}()

	// Check context before starting
	select {
	case <-ctx.Done():
		logger.Debugf("Context cancelled, skipping task for %s", logPath)
		return
	default:
	}

	logger.Debugf("Executing indexing task: %s", logPath)

	// Get processing manager for global state updates
	processingManager := event.GetProcessingStatusManager()

	// Increment active tasks counter and set global status if this is the first task
	isFirstTask := atomic.AddInt32(&ts.activeTasks, 1) == 1
	if isFirstTask {
		processingManager.UpdateNginxLogIndexing(true)
		logger.Debug("Set global indexing status to true")
	}

	// Ensure we always decrement counter and reset global status when no tasks remain
	defer func() {
		remainingTasks := atomic.AddInt32(&ts.activeTasks, -1)
		if remainingTasks == 0 {
			processingManager.UpdateNginxLogIndexing(false)
			logger.Debug("Set global indexing status to false - all tasks completed")
		}
		if r := recover(); r != nil {
			logger.Errorf("Panic during task execution: %v", r)
		}
	}()

	// Set status to indexing
	if err := ts.setTaskStatus(logPath, string(indexer.IndexStatusIndexing), 0); err != nil {
		logger.Errorf("Failed to set indexing status for %s: %v", logPath, err)
		return
	}

	// Execute the indexing with progress tracking
	startTime := time.Now()
	docsCountMap, minTime, maxTime, err := ts.modernIndexer.IndexLogGroupWithProgress(logPath, progressConfig)

	if err != nil {
		logger.Errorf("Failed to execute indexing task %s: %v", logPath, err)
		// Set error status
		if statusErr := ts.setTaskStatus(logPath, string(indexer.IndexStatusError), 0); statusErr != nil {
			logger.Errorf("Failed to set error status for %s: %v", logPath, statusErr)
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
	if err := ts.logFileManager.SaveIndexMetadata(logPath, totalDocsIndexed, startTime, duration, minTime, maxTime); err != nil {
		logger.Errorf("Failed to save index metadata for %s: %v", logPath, err)
	}

	// Set status to indexed (completed)
	if err := ts.setTaskStatus(logPath, string(indexer.IndexStatusIndexed), 0); err != nil {
		logger.Errorf("Failed to set completed status for %s: %v", logPath, err)
	}

	// Update searcher shards
	UpdateSearcherShards()

	logger.Debugf("Successfully completed indexing task: %s, Documents: %d", logPath, totalDocsIndexed)
}

// RecoverUnfinishedTasks recovers indexing tasks that were incomplete at last shutdown
func (ts *TaskScheduler) RecoverUnfinishedTasks(ctx context.Context) error {
	if ts.logFileManager == nil || ts.modernIndexer == nil {
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
		if ts.needsRecovery(log) {
			incompleteTasksCount++

			// Reset to queued status and assign queue position
			if err := ts.recoverTask(ctx, log.Path, queuePosition); err != nil {
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
func (ts *TaskScheduler) needsRecovery(log *NginxLogWithIndex) bool {
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
func (ts *TaskScheduler) recoverTask(ctx context.Context, logPath string, queuePosition int) error {
	// Check if task is already in progress
	if ts.IsTaskInProgress(logPath) {
		logger.Debugf("Skipping recovery for %s - task already in progress", logPath)
		return nil
	}

	logger.Debugf("Recovering indexing task for: %s (queue position: %d)", logPath, queuePosition)

	// Set status to queued with queue position
	if err := ts.setTaskStatus(logPath, string(indexer.IndexStatusQueued), queuePosition); err != nil {
		return err
	}

	// Add a small delay to stagger recovery tasks
	time.Sleep(time.Second * 2)

	// Create recovery progress config
	progressConfig := ts.createProgressConfig()

	// Schedule the task
	return ts.ScheduleIndexTask(ctx, logPath, progressConfig)
}

// createProgressConfig creates a standard progress configuration
func (ts *TaskScheduler) createProgressConfig() *indexer.ProgressConfig {
	return &indexer.ProgressConfig{
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

			logger.Debugf("Indexing progress: %s - %.1f%% (Files: %d/%d, Lines: %d/%d)",
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

			logger.Debugf("Indexing completion: %s - Success: %t, Duration: %s, Lines: %d, Size: %d bytes",
				completion.LogGroupPath, completion.Success, completion.Duration,
				completion.TotalLines, completion.IndexedSize)

			// Send index ready event if indexing was successful
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
}

// setTaskStatus updates the task status in the database
func (ts *TaskScheduler) setTaskStatus(logPath, status string, queuePosition int) error {
	// Get persistence manager
	persistence := ts.logFileManager.GetPersistence()
	if persistence == nil {
		return fmt.Errorf("persistence manager not available")
	}

	// Use enhanced SetIndexStatus method
	return persistence.SetIndexStatus(logPath, status, queuePosition, "")
}

// Shutdown gracefully stops all tasks
func (ts *TaskScheduler) Shutdown() {
	logger.Debug("Shutting down task scheduler...")

	// Cancel all active tasks
	ts.cancel()

	// Wait for all tasks to complete with timeout
	done := make(chan struct{})
	go func() {
		ts.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Debug("All tasks completed successfully")
	case <-time.After(30 * time.Second):
		logger.Warn("Timeout waiting for tasks to complete")
	}

	logger.Debug("Task scheduler shutdown completed")
}
