package event

import (
	"context"

	"github.com/uozi-tech/cosy/logger"
)

// WebSocketHubManager manages WebSocket hub initialization and context handling
type WebSocketHubManager struct {
	ctx    context.Context
	cancel context.CancelFunc
}

var (
	wsHubManager *WebSocketHubManager
)

// InitWebSocketHub initializes the WebSocket hub with proper context handling
func InitWebSocketHub(ctx context.Context) {
	logger.Info("Initializing WebSocket hub...")
	
	hubCtx, cancel := context.WithCancel(ctx)
	wsHubManager = &WebSocketHubManager{
		ctx:    hubCtx,
		cancel: cancel,
	}

	logger.Info("WebSocket hub initialized successfully")

	// Wait for context cancellation
	go func() {
		<-hubCtx.Done()
		logger.Info("WebSocket hub context cancelled")
	}()
}

// GetWebSocketContext returns the WebSocket hub context
func GetWebSocketContext() context.Context {
	if wsHubManager == nil {
		return context.Background()
	}
	return wsHubManager.ctx
}

// ShutdownWebSocketHub gracefully shuts down the WebSocket hub
func ShutdownWebSocketHub() {
	if wsHubManager != nil {
		wsHubManager.cancel()
		logger.Info("WebSocket hub shutdown completed")
	}
}