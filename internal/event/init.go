package event

import (
	"context"

	"github.com/uozi-tech/cosy/logger"
)

// InitEventSystem initializes the event system and sets up WebSocket forwarding
func InitEventSystem(ctx context.Context) {
	logger.Info("Initializing event system...")

	// Initialize the event bus by getting the singleton instance
	GetEventBus()

	// Initialize WebSocket event forwarding configurations
	initWebSocketEventForwarding()

	logger.Info("Event system initialized successfully")
	defer ShutdownEventSystem()

	<-ctx.Done()
}

// initWebSocketEventForwarding initializes WebSocket event forwarding configurations
func initWebSocketEventForwarding() {
	// Register default event forwarding configurations
	RegisterWebSocketEventConfigs(GetDefaultWebSocketEventConfigs())
	logger.Info("WebSocket event forwarding initialized")
}

// ShutdownEventSystem gracefully shuts down the event system
func ShutdownEventSystem() {
	logger.Info("Shutting down event system...")
	GetEventBus().Shutdown()
	logger.Info("Event system shutdown completed")
}
