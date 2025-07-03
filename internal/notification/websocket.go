package notification

import (
	"sync"

	"github.com/0xJacky/Nginx-UI/model"
)

// WebSocketNotificationManager manages WebSocket notification subscriptions
type WebSocketNotificationManager struct {
	subscribers map[chan *model.Notification]struct{}
	mutex       sync.RWMutex
}

var (
	wsManager     *WebSocketNotificationManager
	wsManagerOnce sync.Once
)

// GetWebSocketManager returns the singleton WebSocket notification manager
func GetWebSocketManager() *WebSocketNotificationManager {
	wsManagerOnce.Do(func() {
		wsManager = &WebSocketNotificationManager{
			subscribers: make(map[chan *model.Notification]struct{}),
		}
	})
	return wsManager
}

// Subscribe adds a channel to receive notifications
func (m *WebSocketNotificationManager) Subscribe(ch chan *model.Notification) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.subscribers[ch] = struct{}{}
}

// Unsubscribe removes a channel from receiving notifications
func (m *WebSocketNotificationManager) Unsubscribe(ch chan *model.Notification) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.subscribers, ch)
	close(ch)
}

// Broadcast sends a notification to all subscribers
func (m *WebSocketNotificationManager) Broadcast(data *model.Notification) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for ch := range m.subscribers {
		select {
		case ch <- data:
		default:
			// Skip if channel buffer is full
		}
	}
}

// BroadcastToWebSocket is a convenience function to broadcast notifications
func BroadcastToWebSocket(data *model.Notification) {
	GetWebSocketManager().Broadcast(data)
}
