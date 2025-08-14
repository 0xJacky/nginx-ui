package sites

import (
	"net/http"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

// WebSocket message types
const (
	MessageTypeInitial = "initial"
	MessageTypeUpdate  = "update"
	MessageTypeRefresh = "refresh"
	MessageTypePing    = "ping"
	MessageTypePong    = "pong"
)

// ClientMessage represents incoming WebSocket messages from client
type ClientMessage struct {
	Type string `json:"type"`
}

// ServerMessage represents outgoing WebSocket messages to client
type ServerMessage struct {
	Type string                `json:"type"`
	Data []*sitecheck.SiteInfo `json:"data,omitempty"`
}

// PongMessage represents a pong response
type PongMessage struct {
	Type string `json:"type"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WebSocket connection manager
type WSManager struct {
	connections map[*websocket.Conn]bool
	mutex       sync.RWMutex
}

var wsManager = &WSManager{
	connections: make(map[*websocket.Conn]bool),
}

// AddConnection adds a WebSocket connection to the manager
func (wm *WSManager) AddConnection(conn *websocket.Conn) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()
	wm.connections[conn] = true
}

// RemoveConnection removes a WebSocket connection from the manager
func (wm *WSManager) RemoveConnection(conn *websocket.Conn) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()
	delete(wm.connections, conn)
}

// BroadcastUpdate sends updates to all connected WebSocket clients
func (wm *WSManager) BroadcastUpdate(sites []*sitecheck.SiteInfo) {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	for conn := range wm.connections {
		go func(c *websocket.Conn) {
			if err := sendSiteData(c, MessageTypeUpdate, sites); err != nil {
				logger.Error("Failed to send broadcast update:", err)
				wm.RemoveConnection(c)
				c.Close()
			}
		}(conn)
	}
}

// GetManager returns the global WebSocket manager instance
func GetManager() *WSManager {
	return wsManager
}

// InitWebSocketNotifications sets up the callback for site check updates
func InitWebSocketNotifications() {
	service := sitecheck.GetService()
	service.SetUpdateCallback(func(sites []*sitecheck.SiteInfo) {
		wsManager.BroadcastUpdate(sites)
	})
}

// SiteNavigationWebSocket handles WebSocket connections for real-time site status updates
func SiteNavigationWebSocket(c *gin.Context) {
	ctx := c.Request.Context()

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("WebSocket upgrade failed:", err)
		return
	}
	defer func() {
		wsManager.RemoveConnection(conn)
		conn.Close()
	}()

	logger.Info("Site navigation WebSocket connection established")

	// Register connection with manager
	wsManager.AddConnection(conn)

	service := sitecheck.GetService()

	// Send initial data
	if err := sendSiteData(conn, MessageTypeInitial, service.GetSites()); err != nil {
		logger.Error("Failed to send initial data:", err)
		return
	}

	// Handle incoming messages from client
	go handleClientMessages(conn, service)

	<-ctx.Done()
	logger.Info("Request context cancelled, closing WebSocket")
}

// sendSiteData sends site data via WebSocket
func sendSiteData(conn *websocket.Conn, msgType string, sites []*sitecheck.SiteInfo) error {
	message := ServerMessage{
		Type: msgType,
		Data: sites,
	}
	return conn.WriteJSON(message)
}

// handleClientMessages handles incoming WebSocket messages
func handleClientMessages(conn *websocket.Conn, service *sitecheck.Service) {
	for {
		var msg ClientMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				logger.Error("WebSocket read error:", err)
			}
			return
		}

		switch msg.Type {
		case MessageTypeRefresh:
			logger.Info("Client requested site refresh")
			service.RefreshSites()
		case MessageTypePing:
			pongMsg := PongMessage{Type: MessageTypePong}
			if err := conn.WriteJSON(pongMsg); err != nil {
				logger.Error("Failed to send pong:", err)
				return
			}
		}
	}
}
