package nginx_log

import (
	"context"
	"fmt"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/uozi-tech/cosy/logger"
)

// TaskRecovery handles the recovery of incomplete indexing tasks after restart
type TaskRecovery struct {
	logFileManager *indexer.LogFileManager
	modernIndexer  *indexer.ParallelIndexer
}

// NewTaskRecovery creates a new task recovery manager
func NewTaskRecovery() *TaskRecovery {
	return &TaskRecovery{
		logFileManager: GetLogFileManager(),
		modernIndexer:  GetModernIndexer(),
	}
}

// RecoverUnfinishedTasks recovers indexing tasks that were incomplete at last shutdown
func (tr *TaskRecovery) RecoverUnfinishedTasks(ctx context.Context) error {
	if tr.logFileManager == nil || tr.modernIndexer == nil {
		logger.Warn("Cannot recover tasks: services not available")
		return nil
	}

	logger.Info("Starting recovery of unfinished indexing tasks")

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
		logger.Infof("Recovered %d incomplete indexing tasks", incompleteTasksCount)
	} else {
		logger.Info("No incomplete indexing tasks found")
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
		
	case string(indexer.IndexStatusPartial):
		// Partial indexing should be resumed
		logger.Debugf("Found partial indexing task: %s", log.Path)
		return true
	}
	
	return false
}

// recoverTask recovers a single indexing task
func (tr *TaskRecovery) recoverTask(ctx context.Context, logPath string, queuePosition int) error {
	logger.Infof("Recovering indexing task for: %s (queue position: %d)", logPath, queuePosition)
	
	// Set status to queued with queue position
	if err := tr.setTaskStatus(logPath, string(indexer.IndexStatusQueued), queuePosition); err != nil {
		return err
	}
	
	// Queue the recovery task asynchronously
	go tr.executeRecoveredTask(ctx, logPath)
	
	return nil
}

// executeRecoveredTask executes a recovered indexing task
func (tr *TaskRecovery) executeRecoveredTask(ctx context.Context, logPath string) {
	// Add a small delay to stagger recovery tasks
	time.Sleep(time.Second * 2)
	
	logger.Infof("Executing recovered indexing task: %s", logPath)
	
	// Set status to indexing
	if err := tr.setTaskStatus(logPath, string(indexer.IndexStatusIndexing), 0); err != nil {
		logger.Errorf("Failed to set indexing status for recovered task %s: %v", logPath, err)
		return
	}
	
	// Execute the indexing
	startTime := time.Now()
	docsCountMap, minTime, maxTime, err := tr.modernIndexer.IndexLogGroupWithProgress(logPath, nil)
	
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
	
	logger.Infof("Successfully completed recovered indexing task: %s, Documents: %d", logPath, totalDocsIndexed)
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

// InitTaskRecovery initializes the task recovery system - called during application startup
func InitTaskRecovery(ctx context.Context) {
	logger.Info("Initializing task recovery system")
	
	// Wait a bit for services to fully initialize
	time.Sleep(3 * time.Second)
	
	recoveryManager := NewTaskRecovery()
	if err := recoveryManager.RecoverUnfinishedTasks(ctx); err != nil {
		logger.Errorf("Failed to recover unfinished tasks: %v", err)
	}
}