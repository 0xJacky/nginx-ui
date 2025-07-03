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

// NginxPerformanceClient represents a WebSocket client for Nginx performance monitoring
type NginxPerformanceClient struct {
	conn   *websocket.Conn
	send   chan interface{}
	ctx    context.Context
	cancel context.CancelFunc
	mutex  sync.RWMutex
}

// NginxPerformanceHub manages WebSocket connections for Nginx performance monitoring
type NginxPerformanceHub struct {
	clients    map[*NginxPerformanceClient]bool
	register   chan *NginxPerformanceClient
	unregister chan *NginxPerformanceClient
	mutex      sync.RWMutex
	ticker     *time.Ticker
}

var (
	performanceHub     *NginxPerformanceHub
	performanceHubOnce sync.Once
)

// GetNginxPerformanceHub returns the singleton hub instance
func GetNginxPerformanceHub() *NginxPerformanceHub {
	performanceHubOnce.Do(func() {
		performanceHub = &NginxPerformanceHub{
			clients:    make(map[*NginxPerformanceClient]bool),
			register:   make(chan *NginxPerformanceClient),
			unregister: make(chan *NginxPerformanceClient),
			ticker:     time.NewTicker(5 * time.Second),
		}
		go performanceHub.run()
	})
	return performanceHub
}

// run handles the main hub loop
func (h *NginxPerformanceHub) run() {
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
func (h *NginxPerformanceHub) sendPerformanceDataToClient(client *NginxPerformanceClient) {
	response := performance.GetPerformanceData()

	select {
	case client.send <- response:
	default:
		// Channel is full, remove client
		h.unregister <- client
	}
}

// broadcastPerformanceData sends performance data to all connected clients
func (h *NginxPerformanceHub) broadcastPerformanceData() {
	response := performance.GetPerformanceData()

	h.mutex.RLock()
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

	client := &NginxPerformanceClient{
		conn:   ws,
		send:   make(chan interface{}, 256),
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
func (c *NginxPerformanceClient) writePump() {
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
			return
		}
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *NginxPerformanceClient) readPump() {
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
