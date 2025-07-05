package event

import (
	"context"

	"github.com/uozi-tech/cosy/logger"
)

// InitEventSystem initializes the event system
func InitEventSystem(ctx context.Context) {
	logger.Info("Initializing event system...")

	// Initialize the event bus by getting the singleton instance
	GetEventBus()

	logger.Info("Event system initialized successfully")
	defer ShutdownEventSystem()

	<-ctx.Done()
}

// ShutdownEventSystem gracefully shuts down the event system
func ShutdownEventSystem() {
	logger.Info("Shutting down event system...")
	GetEventBus().Shutdown()
	logger.Info("Event system shutdown completed")
}
