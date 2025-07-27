package upstream

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

// GetAvailability returns cached upstream availability results via HTTP GET
func GetAvailability(c *gin.Context) {
	service := upstream.GetUpstreamService()

	result := gin.H{
		"results":          service.GetAvailabilityMap(),
		"targets":          service.GetTargetInfos(),
		"last_update_time": service.GetLastUpdateTime(),
		"target_count":     service.GetTargetCount(),
	}

	c.JSON(http.StatusOK, result)
}

// GetUpstreamDefinitions returns all upstream definitions for debugging
func GetUpstreamDefinitions(c *gin.Context) {
	service := upstream.GetUpstreamService()

	result := gin.H{
		"upstreams":        service.GetAllUpstreamDefinitions(),
		"last_update_time": service.GetLastUpdateTime(),
	}

	c.JSON(http.StatusOK, result)
}

// AvailabilityWebSocket handles WebSocket connections for real-time availability monitoring
func AvailabilityWebSocket(c *gin.Context) {
	var upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// Upgrade HTTP to WebSocket
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err)
		return
	}
	defer ws.Close()

	// Use context to manage goroutine lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register this connection and increase check frequency
	registerWebSocketConnection()
	defer unregisterWebSocketConnection()

	// Send initial results immediately
	service := upstream.GetUpstreamService()
	initialResults := service.GetAvailabilityMap()
	if err := ws.WriteJSON(initialResults); err != nil {
		logger.Error("Failed to send initial results:", err)
		return
	}

	// Create ticker for periodic updates (every 5 seconds when WebSocket is connected)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Monitor for incoming messages (ping/pong or close)
	go func() {
		defer cancel()
		for {
			// Read message (we don't expect any specific data, just use it for connection health)
			_, _, err := ws.ReadMessage()
			if err != nil {
				if helper.IsUnexpectedWebsocketError(err) {
					logger.Error("WebSocket read error:", err)
				}
				return
			}
		}
	}()

	// Main loop to send periodic updates
	for {
		select {
		case <-ctx.Done():
			logger.Debug("WebSocket connection closed")
			return

		case <-ticker.C:
			// Get latest results from service
			results := service.GetAvailabilityMap()

			// Send results via WebSocket
			if err := ws.WriteJSON(results); err != nil {
				logger.Error("Failed to send WebSocket update:", err)
				if helper.IsUnexpectedWebsocketError(err) {
					return
				}
			}
		case <-kernel.Context.Done():
			logger.Debug("AvailabilityWebSocket: Context cancelled, closing WebSocket")
			return
		}
	}
}

// WebSocket connection tracking for managing check frequency
var (
	wsConnections     int
	wsConnectionMutex sync.Mutex
)

// registerWebSocketConnection increments the WebSocket connection counter
func registerWebSocketConnection() {
	wsConnectionMutex.Lock()
	defer wsConnectionMutex.Unlock()

	wsConnections++
	logger.Debug("WebSocket connection registered, total connections:", wsConnections)

	// Trigger immediate check when first connection is established
	if wsConnections == 1 {
		service := upstream.GetUpstreamService()
		go service.PerformAvailabilityTest()
	}
}

// unregisterWebSocketConnection decrements the WebSocket connection counter
func unregisterWebSocketConnection() {
	wsConnectionMutex.Lock()
	defer wsConnectionMutex.Unlock()

	if wsConnections > 0 {
		wsConnections--
	}
	logger.Debug("WebSocket connection unregistered, remaining connections:", wsConnections)
}
