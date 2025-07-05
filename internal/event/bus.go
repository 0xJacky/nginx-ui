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

// WebSocketEventConfig holds configuration for WebSocket event forwarding
type WebSocketEventConfig struct {
	EventType     EventType
	WSEventName   string
	DataTransform func(data interface{}) interface{}
}

// EventBus manages event publishing and WebSocket forwarding
type EventBus struct {
	wsHub     WebSocketHub
	wsConfigs map[EventType]*WebSocketEventConfig
	wsMutex   sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
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
			wsConfigs: make(map[EventType]*WebSocketEventConfig),
			ctx:       ctx,
			cancel:    cancel,
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

// RegisterWebSocketEventForwarding registers an event type to be forwarded to WebSocket clients
func (eb *EventBus) RegisterWebSocketEventForwarding(eventType EventType, wsEventName string) {
	eb.RegisterWebSocketEventForwardingWithTransform(eventType, wsEventName, func(data interface{}) interface{} {
		return data // Default: no transformation
	})
}

// RegisterWebSocketEventForwardingWithTransform registers an event type with custom data transformation
func (eb *EventBus) RegisterWebSocketEventForwardingWithTransform(eventType EventType, wsEventName string, transform func(data interface{}) interface{}) {
	eb.wsMutex.Lock()
	defer eb.wsMutex.Unlock()

	// Only register if not already registered
	if _, exists := eb.wsConfigs[eventType]; !exists {
		config := &WebSocketEventConfig{
			EventType:     eventType,
			WSEventName:   wsEventName,
			DataTransform: transform,
		}
		eb.wsConfigs[eventType] = config
		logger.Debugf("Registered WebSocket event forwarding: %s -> %s", eventType, wsEventName)
	}
}

// Publish forwards an event directly to WebSocket clients
func (eb *EventBus) Publish(event Event) {
	eb.forwardToWebSocket(event)
}

// forwardToWebSocket forwards an event to WebSocket clients if configured
func (eb *EventBus) forwardToWebSocket(event Event) {
	eb.wsMutex.RLock()
	config, exists := eb.wsConfigs[event.Type]
	hub := eb.wsHub
	eb.wsMutex.RUnlock()

	if !exists || hub == nil {
		return
	}

	// Apply data transformation
	wsData := config.DataTransform(event.Data)
	hub.BroadcastMessage(config.WSEventName, wsData)
}

// Shutdown gracefully shuts down the event bus
func (eb *EventBus) Shutdown() {
	eb.cancel()
	eb.wsMutex.Lock()
	defer eb.wsMutex.Unlock()

	// Clear all configurations
	eb.wsConfigs = make(map[EventType]*WebSocketEventConfig)
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

// RegisterWebSocketEventForwarding registers WebSocket event forwarding on the global bus
func RegisterWebSocketEventForwarding(eventType EventType, wsEventName string) {
	GetEventBus().RegisterWebSocketEventForwarding(eventType, wsEventName)
}

// RegisterWebSocketEventForwardingWithTransform registers WebSocket event forwarding with transform on the global bus
func RegisterWebSocketEventForwardingWithTransform(eventType EventType, wsEventName string, transform func(data interface{}) interface{}) {
	GetEventBus().RegisterWebSocketEventForwardingWithTransform(eventType, wsEventName, transform)
}

// RegisterWebSocketEventConfigs registers multiple WebSocket event configurations
func RegisterWebSocketEventConfigs(configs []WebSocketEventConfig) {
	bus := GetEventBus()
	for _, config := range configs {
		bus.RegisterWebSocketEventForwardingWithTransform(config.EventType, config.WSEventName, config.DataTransform)
	}
}

// GetDefaultWebSocketEventConfigs returns the default WebSocket event configurations
func GetDefaultWebSocketEventConfigs() []WebSocketEventConfig {
	return []WebSocketEventConfig{
		{
			EventType:   EventTypeIndexScanning,
			WSEventName: "index_scanning",
			DataTransform: func(data interface{}) interface{} {
				return data
			},
		},
		{
			EventType:   EventTypeAutoCertProcessing,
			WSEventName: "auto_cert_processing",
			DataTransform: func(data interface{}) interface{} {
				return data
			},
		},
		{
			EventType:   EventTypeProcessingStatus,
			WSEventName: "processing_status",
			DataTransform: func(data interface{}) interface{} {
				return data
			},
		},
		{
			EventType:   EventTypeNginxLogStatus,
			WSEventName: "nginx_log_status",
			DataTransform: func(data interface{}) interface{} {
				return data
			},
		},
		{
			EventType:   EventTypeNotification,
			WSEventName: "notification",
			DataTransform: func(data interface{}) interface{} {
				return data
			},
		},
	}
}
