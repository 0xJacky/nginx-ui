package sites

import (
	"errors"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
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
	CheckOrigin: middleware.CheckWebSocketOrigin,
}

// WSManager WebSocket connection manager
type WSManager struct {
	connections map[*websocket.Conn]*WSClient
	mutex       sync.RWMutex
}

var errClientUnavailable = errors.New("websocket client unavailable")

// WSClient wraps a websocket connection and handles serialized writes.
type WSClient struct {
	conn   *websocket.Conn
	send   chan interface{}
	mutex  sync.RWMutex
	closed bool
}

func (c *WSClient) trySend(v interface{}) bool {
	c.mutex.RLock()
	if c.closed {
		c.mutex.RUnlock()
		return false
	}

	select {
	case c.send <- v:
		c.mutex.RUnlock()
		return true
	default:
		c.mutex.RUnlock()
		return false
	}
}

func (c *WSClient) closeSendChannel() {
	c.mutex.Lock()
	if c.closed {
		c.mutex.Unlock()
		return
	}

	close(c.send)
	c.closed = true
	c.mutex.Unlock()
}

func (c *WSClient) writePump() {
	for message := range c.send {
		c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := c.conn.WriteJSON(message); err != nil {
			logger.Error("Failed to write site websocket message:", err)
			return
		}
	}
}

var wsManager = &WSManager{
	connections: make(map[*websocket.Conn]*WSClient),
}

// AddConnection adds a WebSocket connection to the manager
func (wm *WSManager) AddConnection(conn *websocket.Conn) *WSClient {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()
	client := &WSClient{
		conn: conn,
		send: make(chan interface{}, 16),
	}
	wm.connections[conn] = client
	return client
}

// RemoveConnection removes a WebSocket connection from the manager
func (wm *WSManager) RemoveConnection(conn *websocket.Conn) {
	wm.mutex.Lock()
	client, ok := wm.connections[conn]
	if ok {
		delete(wm.connections, conn)
	}
	wm.mutex.Unlock()

	if ok {
		client.closeSendChannel()
	}
}

func (wm *WSManager) activeClients() []*WSClient {
	wm.mutex.RLock()
	if len(wm.connections) == 0 {
		wm.mutex.RUnlock()
		return nil
	}

	clients := make([]*WSClient, 0, len(wm.connections))
	for _, client := range wm.connections {
		clients = append(clients, client)
	}
	wm.mutex.RUnlock()

	return clients
}

// BroadcastUpdate sends updates to all connected WebSocket clients
func (wm *WSManager) BroadcastUpdate(sites []*sitecheck.SiteInfo) {
	for _, client := range wm.activeClients() {
		if err := sendSiteData(client, MessageTypeUpdate, sites); err == nil {
			continue
		}

		wm.RemoveConnection(client.conn)
		client.conn.Close()
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
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("WebSocket upgrade failed:", err)
		return
	}

	client := wsManager.AddConnection(conn)
	defer func() {
		wsManager.RemoveConnection(conn)
		conn.Close()
	}()

	logger.Info("Site navigation WebSocket connection established")

	service := sitecheck.GetService()

	go client.writePump()

	// Send initial data
	if err := sendSiteData(client, MessageTypeInitial, service.GetSites()); err != nil {
		logger.Error("Failed to queue initial site data:", err)
		return
	}

	handleClientMessages(client, service)
	logger.Info("Site navigation WebSocket connection closed")
}

// sendSiteData sends site data via WebSocket
func sendSiteData(client *WSClient, msgType string, sites []*sitecheck.SiteInfo) error {
	message := ServerMessage{
		Type: msgType,
		Data: sites,
	}

	if !client.trySend(message) {
		return errClientUnavailable
	}

	return nil
}

// handleClientMessages handles incoming WebSocket messages
func handleClientMessages(client *WSClient, service *sitecheck.Service) {
	for {
		var msg ClientMessage
		if err := client.conn.ReadJSON(&msg); err != nil {
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
			if !client.trySend(pongMsg) {
				logger.Error("Failed to queue pong response:", errClientUnavailable)
				return
			}
		}
	}
}
