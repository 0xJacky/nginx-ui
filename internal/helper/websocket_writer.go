package helper

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SafeWebSocketWriter serializes writes for a websocket connection.
type SafeWebSocketWriter struct {
	conn  *websocket.Conn
	mutex sync.Mutex
}

// NewSafeWebSocketWriter creates a serialized writer for a websocket connection.
func NewSafeWebSocketWriter(conn *websocket.Conn) *SafeWebSocketWriter {
	return &SafeWebSocketWriter{conn: conn}
}

// WriteJSON writes JSON data with serialized access to the websocket connection.
func (w *SafeWebSocketWriter) WriteJSON(v interface{}) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return w.conn.WriteJSON(v)
}

// WriteMessage writes a websocket message with serialized access to the connection.
func (w *SafeWebSocketWriter) WriteMessage(messageType int, data []byte) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return w.conn.WriteMessage(messageType, data)
}
