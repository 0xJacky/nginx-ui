package nginx_log

import (
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/uozi-tech/cosy/logger"
)

// IndexingStatusManager manages the global indexing status
type IndexingStatusManager struct {
	mu       sync.RWMutex
	indexing bool
}

var (
	statusManager *IndexingStatusManager
	statusOnce    sync.Once
)

// GetIndexingStatusManager returns the singleton instance of IndexingStatusManager
func GetIndexingStatusManager() *IndexingStatusManager {
	statusOnce.Do(func() {
		statusManager = &IndexingStatusManager{
			indexing: false,
		}
	})
	return statusManager
}

// UpdateIndexingStatus updates the global indexing status based on current file states
func (m *IndexingStatusManager) UpdateIndexingStatus() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if any files are currently being indexed
	indexingFiles := GetIndexingFiles()
	newIndexingStatus := len(indexingFiles) > 0

	// Only publish event if status changed
	if m.indexing != newIndexingStatus {
		m.indexing = newIndexingStatus
		
		logger.Infof("Global indexing status changed to: %t (active files: %d)", 
			newIndexingStatus, len(indexingFiles))

		// Update global processing status
		processingManager := event.GetProcessingStatusManager()
		processingManager.UpdateNginxLogIndexing(newIndexingStatus)
	}
}

// IsIndexing returns the current global indexing status
func (m *IndexingStatusManager) IsIndexing() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.indexing
}

// NotifyFileIndexingStarted should be called when a file starts indexing
func (m *IndexingStatusManager) NotifyFileIndexingStarted(filePath string) {
	logger.Infof("File indexing started: %s", filePath)
	SetIndexingStatus(filePath, true)
	m.UpdateIndexingStatus()
}

// NotifyFileIndexingCompleted should be called when a file finishes indexing
func (m *IndexingStatusManager) NotifyFileIndexingCompleted(filePath string) {
	logger.Infof("File indexing completed: %s", filePath)
	SetIndexingStatus(filePath, false)
	m.UpdateIndexingStatus()
}