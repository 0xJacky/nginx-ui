package cert

import (
	"context"
	"sync"
)

var (
	// mutex is used to control access to certificate operations
	mutex sync.Mutex

	// statusChan is the channel to broadcast certificate status changes
	statusChan = make(chan bool, 10)

	// subscribers is a map of channels that are subscribed to certificate status changes
	subscribers = make(map[chan bool]struct{})

	// subscriberMux protects the subscribers map from concurrent access
	subscriberMux sync.RWMutex

	// isProcessing indicates whether a certificate operation is in progress
	isProcessing bool

	// processingMutex protects the isProcessing flag
	processingMutex sync.RWMutex
)

func initBroadcastStatus(ctx context.Context) {
	// Start broadcasting goroutine
	go broadcastStatus(ctx)
}

// broadcastStatus listens for status changes and broadcasts to all subscribers
func broadcastStatus(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Context cancelled, clean up resources and exit
			close(statusChan)
			return
		case status, ok := <-statusChan:
			if !ok {
				// Channel closed, exit
				return
			}
			subscriberMux.RLock()
			for ch := range subscribers {
				// Non-blocking send to prevent slow subscribers from blocking others
				select {
				case ch <- status:
				default:
					// Skip if channel buffer is full
				}
			}
			subscriberMux.RUnlock()
		}
	}
}

// SubscribeProcessingStatus allows a client to subscribe to certificate processing status changes
func SubscribeProcessingStatus() chan bool {
	ch := make(chan bool, 5) // Buffer to prevent blocking

	// Add to subscribers
	subscriberMux.Lock()
	subscribers[ch] = struct{}{}
	subscriberMux.Unlock()

	// Send current status immediately
	processingMutex.RLock()
	currentStatus := isProcessing
	processingMutex.RUnlock()

	// Non-blocking send
	select {
	case ch <- currentStatus:
	default:
	}

	return ch
}

// UnsubscribeProcessingStatus removes a subscriber from receiving status updates
func UnsubscribeProcessingStatus(ch chan bool) {
	subscriberMux.Lock()
	delete(subscribers, ch)
	subscriberMux.Unlock()

	// Close the channel so the client knows it's unsubscribed
	close(ch)
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

// setProcessingStatus updates the processing status and broadcasts the change
func setProcessingStatus(status bool) {
	processingMutex.Lock()
	if isProcessing != status {
		isProcessing = status
		statusChan <- status
	}
	processingMutex.Unlock()
}
