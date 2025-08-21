package nginx_log

import (
	"context"
	"sync"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

// debounceIndexTask implements file-level debouncing for index operations
func (li *LogIndexer) debounceIndexTask(task *IndexTask) {
	// First, check if the context is already cancelled
	select {
	case <-li.ctx.Done():
		logger.Debugf("Debounce check cancelled for %s as context is done.", task.FilePath)
		if task.Wg != nil {
			task.Wg.Done()
		}
		return
	default:
	}

	filePath := task.FilePath

	// Check if we need to respect the minimum interval
	if lastTime, exists := li.lastIndexTime.Load(filePath); exists {
		if lastIndexTime, ok := lastTime.(time.Time); ok {
			timeSinceLastIndex := time.Since(lastIndexTime)
			if timeSinceLastIndex < MinIndexInterval {
				// Calculate remaining wait time
				remainingWait := MinIndexInterval - timeSinceLastIndex

				// Cancel any existing timer for this file
				if timerInterface, exists := li.debounceTimers.Load(filePath); exists {
					if timer, ok := timerInterface.(*time.Timer); ok {
						timer.Stop()
					}
				}

				// Set new timer
				timer := time.AfterFunc(remainingWait, func() {
					// Clean up timer
					li.debounceTimers.Delete(filePath)
					// Execute the actual indexing
					li.executeIndexTask(task)
				})

				li.debounceTimers.Store(filePath, timer)
				return
			}
		}
	}

	// No debouncing needed, execute immediately
	li.executeIndexTask(task)
}

// executeIndexTask executes the actual indexing task and updates last index time
func (li *LogIndexer) executeIndexTask(task *IndexTask) {
	// Update last index time before processing
	li.lastIndexTime.Store(task.FilePath, time.Now())

	// Check if context is still valid
	select {
	case <-li.ctx.Done():
		logger.Warnf("Index task cancelled for file: %s", task.FilePath)
		return
	default:
	}

	// Queue the task for processing
	select {
	case li.indexQueue <- task:
		// Task queued successfully (no debug log to avoid spam)
	default:
		logger.Warnf("Index queue is full, dropping task for file: %s", task.FilePath)
		// If there's a WaitGroup, we must decrement it to avoid deadlock
		if task.Wg != nil {
			task.Wg.Done()
		}
	}
}

// processIndexTask processes a single indexing task with file locking
func (li *LogIndexer) processIndexTask(task *IndexTask) {
	// Ensure WaitGroup is handled correctly
	if task.Wg != nil {
		defer task.Wg.Done()
	}

	// Get or create a mutex for this file
	mutexInterface, _ := li.indexingLock.LoadOrStore(task.FilePath, &sync.Mutex{})
	fileMutex := mutexInterface.(*sync.Mutex)

	// Lock the file for indexing
	fileMutex.Lock()
	defer fileMutex.Unlock()

	logger.Infof("Processing index task for file: %s (priority: %d, full_reindex: %v)", task.FilePath, task.Priority, task.FullReindex)

	// Create a context with timeout for this task
	ctx, cancel := context.WithTimeout(li.ctx, 10*time.Minute)
	defer cancel()

	// Check if context is still valid
	select {
	case <-ctx.Done():
		logger.Warnf("Index task cancelled for file: %s", task.FilePath)
		return
	default:
	}

	// Perform the actual indexing
	if err := li.IndexLogFileWithMode(task.FilePath, task.FullReindex); err != nil {
		logger.Errorf("Failed to index file %s: %v", task.FilePath, err)
	} else {
		logger.Infof("Successfully indexed file: %s", task.FilePath)
		// Note: Log group notifications are handled centrally after all files complete
	}
}
