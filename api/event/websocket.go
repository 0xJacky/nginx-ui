package event

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

// WebSocketMessage represents the structure of messages sent to the client
type WebSocketMessage struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

// Client represents a WebSocket client connection
type Client struct {
	conn   *websocket.Conn
	send   chan WebSocketMessage
	ctx    context.Context
	cancel context.CancelFunc
	mutex  sync.RWMutex
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

		// Register this hub directly with the event bus
		event.SetWebSocketHub(hub)
	})
	return hub
}

// BroadcastMessage implements the WebSocketHub interface
func (h *Hub) BroadcastMessage(event string, data interface{}) {
	message := WebSocketMessage{
		Event: event,
		Data:  data,
	}
	select {
	case h.broadcast <- message:
	case <-time.After(1 * time.Second):
		logger.Warn("Broadcast channel full, message dropped after timeout", "event", event)
	default:
		logger.Warn("Broadcast channel full, message dropped immediately", "event", event)
	}
}

// run handles the main hub loop
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			logger.Debug("Client connected, total clients:", len(h.clients))

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mutex.Unlock()
			logger.Debug("Client disconnected, total clients:", len(h.clients))

		case message := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mutex.RUnlock()
		}
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

// EventBus handles the main WebSocket connection for the event bus
func EventBus(c *gin.Context) {
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

	// Start write and read pumps - no manual event subscriptions needed
	go client.writePump()
	client.readPump()
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				logger.Error("Failed to write message:", err)
				if helper.IsUnexpectedWebsocketError(err) {
					return
				}
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Error("Failed to write ping:", err)
				return
			}

		case <-c.ctx.Done():
			return

		case <-kernel.Context.Done():
			logger.Debug("EventBus: Context cancelled, closing WebSocket")
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

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg json.RawMessage
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				logger.Error("Unexpected WebSocket error:", err)
			}
			break
		}
		// Handle incoming messages if needed
		// For now, this is a one-way communication (server to client)
	}
}
