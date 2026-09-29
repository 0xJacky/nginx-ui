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
				Plugins:            []PluginActivity{},
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

// MaxPluginActivities bounds the entries one plugin may show at a time.
const MaxPluginActivities = 16

// SetPluginActivity shows or updates one entry of a plugin, or removes it when
// active is false. It returns false when a new entry would exceed
// MaxPluginActivities.
func (m *ProcessingStatusManager) SetPluginActivity(pluginID, key, label string, active bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	current := m.status.Plugins
	index, count := -1, 0
	for i, item := range current {
		if item.PluginID != pluginID {
			continue
		}
		count++
		if item.Key == key {
			index = i
		}
	}

	// The list is published to subscribers, so it is replaced, never edited.
	next := make([]PluginActivity, 0, len(current)+1)
	switch {
	case !active:
		if index < 0 {
			return true
		}
		next = append(next, current[:index]...)
		next = append(next, current[index+1:]...)
	case index >= 0:
		if current[index].Label == label {
			return true
		}
		next = append(next, current...)
		next[index].Label = label
	default:
		if count >= MaxPluginActivities {
			return false
		}
		next = append(next, current...)
		next = append(next, PluginActivity{PluginID: pluginID, Key: key, Label: label})
	}

	m.status.Plugins = next
	m.publishStatus()
	return true
}

// ClearPluginActivities removes every entry of a plugin.
func (m *ProcessingStatusManager) ClearPluginActivities(pluginID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	next := make([]PluginActivity, 0, len(m.status.Plugins))
	for _, item := range m.status.Plugins {
		if item.PluginID != pluginID {
			next = append(next, item)
		}
	}
	if len(next) == len(m.status.Plugins) {
		return
	}
	m.status.Plugins = next
	m.publishStatus()
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
