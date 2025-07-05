package cert

import (
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/event"
)

var (
	// mutex is used to control access to certificate operations
	mutex sync.Mutex

	// isProcessing indicates whether a certificate operation is in progress
	isProcessing bool

	// processingMutex protects the isProcessing flag
	processingMutex sync.RWMutex
)

// publishProcessingStatus publishes the processing status to the event bus
func publishProcessingStatus(processing bool) {
	event.Publish(event.Event{
		Type: event.EventTypeAutoCertProcessing,
		Data: processing,
	})
}

// lock acquires the certificate mutex
func lock() {
	mutex.Lock()
	setProcessingStatus(true)
}

// unlock releases the certificate mutex
func unlock() {
	setProcessingStatus(false)
	mutex.Unlock()
}

// IsProcessing returns whether a certificate operation is currently in progress
func IsProcessing() bool {
	processingMutex.RLock()
	defer processingMutex.RUnlock()
	return isProcessing
}

// setProcessingStatus updates the processing status and publishes the change
func setProcessingStatus(status bool) {
	processingMutex.Lock()
	if isProcessing != status {
		isProcessing = status
		publishProcessingStatus(status)
	}
	processingMutex.Unlock()
}
