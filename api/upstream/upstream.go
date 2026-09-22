package upstream

import (
	"net/http"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
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

// availabilityPushInterval is how often availability results are pushed while
// a WebSocket client is connected.
const availabilityPushInterval = 5 * time.Second

// AvailabilityWebSocket handles WebSocket connections for real-time availability monitoring
func AvailabilityWebSocket(c *gin.Context) {
	var upGrader = websocket.Upgrader{
		CheckOrigin: middleware.CheckWebSocketOrigin,
	}

	// Upgrade HTTP to WebSocket
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err)
		return
	}
	defer ws.Close()

	// This handler only pushes. The keepalive owns the reader, so a peer that
	// vanished without a close frame ends the session within the pong wait
	// instead of holding a wsConnections slot until TCP gives up.
	keepalive := helper.StartWebSocketKeepaliveReader(ws)
	defer keepalive.Stop()

	// Register this connection and increase check frequency
	registerWebSocketConnection()
	defer unregisterWebSocketConnection()

	// Send initial results immediately
	service := upstream.GetUpstreamService()
	if !writeAvailability(ws, service.GetAvailabilityMap()) {
		return
	}

	ticker := time.NewTicker(availabilityPushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-keepalive.Done():
			logger.Debug("AvailabilityWebSocket: peer disconnected, closing WebSocket")
			return

		case <-ticker.C:
			if !writeAvailability(ws, service.GetAvailabilityMap()) {
				return
			}

		case <-kernel.Context.Done():
			logger.Debug("AvailabilityWebSocket: Context cancelled, closing WebSocket")
			return
		}
	}
}

// writeAvailability pushes one availability snapshot and reports whether the
// handler should keep pushing. A failed or timed-out write leaves the
// connection unusable, so every error ends the session.
func writeAvailability(ws *websocket.Conn, results map[string]*upstream.Status) bool {
	_ = ws.SetWriteDeadline(time.Now().Add(helper.WebSocketWriteWait))
	if err := ws.WriteJSON(results); err != nil {
		if helper.IsUnexpectedWebsocketError(err) {
			logger.Error("Failed to send WebSocket update:", err)
		}
		return false
	}
	return true
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
