package event

import (
	"context"
	"sync"

	"github.com/uozi-tech/cosy/logger"
)

// WebSocketHub interface for broadcasting messages
type WebSocketHub interface {
	BroadcastMessage(event string, data interface{})
}

// EventBus manages event publishing and WebSocket forwarding
type EventBus struct {
	wsHub   WebSocketHub
	wsMutex sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

var (
	globalBus *EventBus
	busOnce   sync.Once
)

// GetEventBus returns the global event bus instance
func GetEventBus() *EventBus {
	busOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		globalBus = &EventBus{
			ctx:    ctx,
			cancel: cancel,
		}
	})
	return globalBus
}

// SetWebSocketHub sets the WebSocket hub for direct event forwarding
func (eb *EventBus) SetWebSocketHub(hub WebSocketHub) {
	eb.wsMutex.Lock()
	defer eb.wsMutex.Unlock()
	eb.wsHub = hub
	logger.Info("WebSocket hub registered with event bus")
}

// Publish forwards an event directly to WebSocket clients
func (eb *EventBus) Publish(event Event) {
	eb.wsMutex.RLock()
	hub := eb.wsHub
	eb.wsMutex.RUnlock()

	if hub == nil {
		return
	}

	// Directly broadcast the event using its type as the event name
	hub.BroadcastMessage(string(event.Type), event.Data)
}

// Shutdown gracefully shuts down the event bus
func (eb *EventBus) Shutdown() {
	eb.cancel()
	eb.wsMutex.Lock()
	defer eb.wsMutex.Unlock()

	eb.wsHub = nil
	logger.Info("Event bus shutdown completed")
}

// Context returns the event bus context
func (eb *EventBus) Context() context.Context {
	return eb.ctx
}

// Convenience functions for global event bus

// Publish forwards an event to WebSocket clients on the global bus
func Publish(event Event) {
	GetEventBus().Publish(event)
}

// SetWebSocketHub sets the WebSocket hub for the global event bus
func SetWebSocketHub(hub WebSocketHub) {
	GetEventBus().SetWebSocketHub(hub)
}
