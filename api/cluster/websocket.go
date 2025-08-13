package cluster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

// WebSocketMessage represents the structure of messages sent to the client
type WebSocketMessage struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

// Client represents a WebSocket client connection for cluster node monitoring
type Client struct {
	conn   *websocket.Conn
	send   chan WebSocketMessage
	ctx    context.Context
	cancel context.CancelFunc
}

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan WebSocketMessage
	register   chan *Client
	unregister chan *Client
	mutex      sync.RWMutex
}

var (
	hub     *Hub
	hubOnce sync.Once
)

// GetHub returns the singleton hub instance
func GetHub() *Hub {
	hubOnce.Do(func() {
		hub = &Hub{
			clients:    make(map[*Client]bool),
			broadcast:  make(chan WebSocketMessage, 1024), // Increased buffer size
			register:   make(chan *Client),
			unregister: make(chan *Client),
		}
		go hub.run()
	})
	return hub
}

// run handles the main hub loop
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			logger.Debug("Cluster node client connected, total clients:", len(h.clients))

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mutex.Unlock()
			logger.Debug("Cluster node client disconnected, total clients:", len(h.clients))

		case message := <-h.broadcast:
			h.mutex.RLock()
			deadClients := make([]*Client, 0)
			for client := range h.clients {
				select {
				case client.send <- message:
				case <-time.After(100 * time.Millisecond):
					// Client is too slow, mark for removal
					logger.Debug("Client send channel timeout, marking for removal")
					deadClients = append(deadClients, client)
				default:
					// Channel is full, mark for removal
					logger.Debug("Client send channel full, marking for removal")
					deadClients = append(deadClients, client)
				}
			}
			h.mutex.RUnlock()

			// Clean up dead clients
			if len(deadClients) > 0 {
				h.mutex.Lock()
				for _, client := range deadClients {
					if _, ok := h.clients[client]; ok {
						close(client.send)
						delete(h.clients, client)
						client.cancel() // Trigger client cleanup
					}
				}
				h.mutex.Unlock()
				logger.Info("Cleaned up slow/unresponsive clients", "count", len(deadClients))
			}
		}
	}
}

// BroadcastMessage sends a message to all connected clients
func (h *Hub) BroadcastMessage(event string, data any) {
	message := WebSocketMessage{
		Event: event,
		Data:  data,
	}
	select {
	case h.broadcast <- message:
	default:
		logger.Warn("Cluster node broadcast channel full, message dropped")
	}
}

// WebSocket upgrader configuration
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type respNode struct {
	*model.Node
	Status bool `json:"status"`
}

// GetAllEnabledNodeWS handles WebSocket connections for real-time node monitoring
func GetAllEnabledNodeWS(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("Failed to upgrade connection:", err)
		return
	}
	defer ws.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &Client{
		conn:   ws,
		send:   make(chan WebSocketMessage, 1024), // Increased buffer size
		ctx:    ctx,
		cancel: cancel,
	}

	hub := GetHub()
	hub.register <- client

	// Start goroutines for handling node monitoring
	go client.handleNodeMonitoring()

	// Start write and read pumps
	go client.writePump()
	client.readPump()
}

// handleNodeMonitoring monitors node status and sends updates
func (c *Client) handleNodeMonitoring() {
	interval := 10 * time.Second
	heartbeatInterval := 30 * time.Second

	getNodeData := func() (interface{}, bool) {
		// Query nodes directly from database
		var nodes []model.Node
		err := model.UseDB().Where("enabled = ?", true).Find(&nodes).Error
		if err != nil {
			logger.Error("Failed to query nodes:", err)
			return nil, false
		}

		// Transform nodes to response format
		var result []respNode
		for _, node := range nodes {
			result = append(result, respNode{
				Node:   &node,
				Status: analytic.GetNode(&node).Status,
			})
		}

		return result, true
	}

	getHash := func(data interface{}) string {
		bytes, _ := json.Marshal(data)
		hash := sha256.New()
		hash.Write(bytes)
		hashSum := hash.Sum(nil)
		return hex.EncodeToString(hashSum)
	}

	var dataHash string

	// Send initial data
	data, ok := getNodeData()
	if ok {
		dataHash = getHash(data)
		c.sendMessage("message", data)
	}

	ticker := time.NewTicker(interval)
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ticker.C:
			data, ok := getNodeData()
			if !ok {
				return
			}

			newHash := getHash(data)
			if dataHash != newHash {
				dataHash = newHash
				c.sendMessage("message", data)
			}

		case <-heartbeatTicker.C:
			c.sendMessage("heartbeat", "")

		case <-c.ctx.Done():
			return
		}
	}
}

// sendMessage sends a message to the client with timeout and better error handling
func (c *Client) sendMessage(event string, data any) {
	message := WebSocketMessage{
		Event: event,
		Data:  data,
	}

	select {
	case c.send <- message:
	case <-time.After(5 * time.Second):
		logger.Warn("Client send channel full, message dropped after timeout", "event", event)
		// Force disconnect slow clients to prevent resource leakage
		c.cancel()
	default:
		logger.Warn("Client send channel full, message dropped immediately", "event", event)
		// For non-critical messages, we can drop them immediately
		if event != "heartbeat" {
			logger.Info("Dropping non-critical message due to full channel", "event", event)
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				logger.Error("Error writing message to websocket:", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-kernel.Context.Done():
			return
		case <-c.ctx.Done():
			return
		}
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		hub := GetHub()
		hub.unregister <- c
		c.conn.Close()
		c.cancel()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	go func() {
		for {
			_, _, err := c.conn.ReadMessage()
			if err != nil {
				if helper.IsUnexpectedWebsocketError(err) {
					logger.Error("Websocket error:", err)
				}
				return
			}
		}
	}()

	select {
	case <-kernel.Context.Done():
		return
	case <-c.ctx.Done():
		return
	}
}
