package event

import (
	"sync"

	"github.com/uozi-tech/cosy/logger"
)

// ProcessingStatusManager manages the global processing status
type ProcessingStatusManager struct {
	mu     sync.RWMutex
	status ProcessingStatusData
}

var (
	processingManager *ProcessingStatusManager
	processingOnce    sync.Once
)

// GetProcessingStatusManager returns the singleton instance of ProcessingStatusManager
func GetProcessingStatusManager() *ProcessingStatusManager {
	processingOnce.Do(func() {
		processingManager = &ProcessingStatusManager{
			status: ProcessingStatusData{
				IndexScanning:      false,
				AutoCertProcessing: false,
				NginxLogIndexing:   false,
			},
		}
	})
	return processingManager
}

// GetCurrentStatus returns the current processing status
func (m *ProcessingStatusManager) GetCurrentStatus() ProcessingStatusData {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// UpdateIndexScanning updates the index scanning status
func (m *ProcessingStatusManager) UpdateIndexScanning(scanning bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status.IndexScanning != scanning {
		m.status.IndexScanning = scanning
		logger.Infof("Index scanning status changed to: %t", scanning)
		m.publishStatus()
	}
}

// UpdateAutoCertProcessing updates the auto cert processing status
func (m *ProcessingStatusManager) UpdateAutoCertProcessing(processing bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status.AutoCertProcessing != processing {
		m.status.AutoCertProcessing = processing
		logger.Infof("Auto cert processing status changed to: %t", processing)
		m.publishStatus()
	}
}

// UpdateNginxLogIndexing updates the nginx log indexing status
func (m *ProcessingStatusManager) UpdateNginxLogIndexing(indexing bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status.NginxLogIndexing != indexing {
		m.status.NginxLogIndexing = indexing
		logger.Infof("Nginx log indexing status changed to: %t", indexing)
		m.publishStatus()

		// Also publish legacy nginx_log_status for backward compatibility
		Publish(Event{
			Type: TypeNginxLogStatus,
			Data: NginxLogStatusData{
				Indexing: indexing,
			},
		})
	}
}

// publishStatus publishes the current processing status
func (m *ProcessingStatusManager) publishStatus() {
	Publish(Event{
		Type: TypeProcessingStatus,
		Data: m.status,
	})
}

// BroadcastCurrentStatus broadcasts the current status (used when clients connect)
func (m *ProcessingStatusManager) BroadcastCurrentStatus() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	logger.Debug("Broadcasting current processing status to new client")
	Publish(Event{
		Type: TypeProcessingStatus,
		Data: m.status,
	})
}
