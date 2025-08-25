package nginx

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/0xJacky/Nginx-UI/internal/performance"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

// PerformanceClient represents a WebSocket client for Nginx performance monitoring
type PerformanceClient struct {
	conn   *websocket.Conn
	send   chan interface{}
	ctx    context.Context
	cancel context.CancelFunc
	mutex  sync.RWMutex
}

// PerformanceHub manages WebSocket connections for Nginx performance monitoring
type PerformanceHub struct {
	clients    map[*PerformanceClient]bool
	register   chan *PerformanceClient
	unregister chan *PerformanceClient
	mutex      sync.RWMutex
	ticker     *time.Ticker
}

var (
	performanceHub     *PerformanceHub
	performanceHubOnce sync.Once
)

// GetNginxPerformanceHub returns the singleton hub instance
func GetNginxPerformanceHub() *PerformanceHub {
	performanceHubOnce.Do(func() {
		performanceHub = &PerformanceHub{
			clients:    make(map[*PerformanceClient]bool),
			register:   make(chan *PerformanceClient),
			unregister: make(chan *PerformanceClient),
			ticker:     time.NewTicker(5 * time.Second),
		}
		go performanceHub.run()
	})
	return performanceHub
}

// run handles the main hub loop
func (h *PerformanceHub) run() {
	defer h.ticker.Stop()

	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			logger.Debug("Nginx performance client connected, total clients:", len(h.clients))

			// Send initial data to the new client
			go h.sendPerformanceDataToClient(client)

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mutex.Unlock()
			logger.Debug("Nginx performance client disconnected, total clients:", len(h.clients))

		case <-h.ticker.C:
			// Send performance data to all connected clients
			h.broadcastPerformanceData()

		case <-kernel.Context.Done():
			logger.Debug("PerformanceHub: Context cancelled, closing WebSocket")
			// Shutdown all clients
			h.mutex.Lock()
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			h.mutex.Unlock()
			return
		}
	}
}

// sendPerformanceDataToClient sends performance data to a specific client
func (h *PerformanceHub) sendPerformanceDataToClient(client *PerformanceClient) {
	response := performance.GetPerformanceData()

	select {
	case client.send <- response:
	default:
		// Channel is full, remove client
		h.unregister <- client
	}
}

// broadcastPerformanceData sends performance data to all connected clients
func (h *PerformanceHub) broadcastPerformanceData() {
	h.mutex.RLock()

	// Check if there are any connected clients
	if len(h.clients) == 0 {
		h.mutex.RUnlock()
		return
	}

	// Only get performance data if there are connected clients
	response := performance.GetPerformanceData()

	for client := range h.clients {
		select {
		case client.send <- response:
		default:
			// Channel is full, remove client
			close(client.send)
			delete(h.clients, client)
		}
	}
	h.mutex.RUnlock()
}

// WebSocket upgrader configuration
var nginxPerformanceUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// StreamDetailStatusWS handles WebSocket connection for Nginx performance monitoring
func StreamDetailStatusWS(c *gin.Context) {
	ws, err := nginxPerformanceUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("Failed to upgrade connection:", err)
		return
	}
	defer ws.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &PerformanceClient{
		conn:   ws,
		send:   make(chan interface{}, 1024), // Increased buffer size
		ctx:    ctx,
		cancel: cancel,
	}

	hub := GetNginxPerformanceHub()
	hub.register <- client

	// Start write and read pumps
	go client.writePump()
	client.readPump()
}

// writePump pumps messages from the hub to the websocket connection
func (c *PerformanceClient) writePump() {
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
			logger.Debug("PerformanceClient: Context cancelled, closing WebSocket")
			return
		}
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *PerformanceClient) readPump() {
	defer func() {
		hub := GetNginxPerformanceHub()
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
		_, _, err := c.conn.ReadMessage()
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
