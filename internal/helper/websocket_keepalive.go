package helper

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

// WebSocketWriteWait bounds a single frame write. Data writes should set it as
// their write deadline so a stalled peer cannot hold the connection's write
// lock, which pings also need, indefinitely.
const WebSocketWriteWait = 10 * time.Second

// WebSocketKeepaliveConfig controls how a WebSocketKeepalive probes its peer.
type WebSocketKeepaliveConfig struct {
	// PingPeriod is the interval between ping frames. It must be shorter than
	// PongWait so a live peer can answer before the read deadline expires.
	PingPeriod time.Duration
	// PongWait is how long the connection may stay silent, with neither a pong
	// nor a data message arriving, before the peer is considered gone.
	PongWait time.Duration
	// WriteWait bounds each ping write.
	WriteWait time.Duration
}

// WebSocketKeepaliveDefaults is the timing used by StartWebSocketKeepalive and
// StartWebSocketKeepaliveReader. It matches the event, nginx and cluster
// sockets: a ping every 30s, a 60s pong wait and a 10s write wait.
//
// Production code must treat it as read-only. Tests may shorten it before
// starting a handler and restore it once that handler has returned.
var WebSocketKeepaliveDefaults = WebSocketKeepaliveConfig{
	PingPeriod: 30 * time.Second,
	PongWait:   60 * time.Second,
	WriteWait:  WebSocketWriteWait,
}

// WebSocketKeepalive detects a peer that disappeared without a close frame,
// for example a closed laptop lid, a network switch or a tab killed behind a
// NAT. Without it a handler only notices when a write finally fails or TCP
// gives up, which can take many minutes.
//
// It arms a read deadline that every pong, and every read made through the
// keepalive, pushes forward. It pings the peer from its own goroutine through
// WriteControl, which gorilla/websocket allows concurrently with the
// connection's single reader and single writer. Browsers answer pings
// automatically, so an idle but live client is never disconnected.
//
// Pong frames are only processed while something reads the connection, so a
// keepalive always needs a reader:
//
//   - Handlers that run their own read loop use StartWebSocketKeepalive and
//     read through ReadMessage or ReadJSON. Once the peer goes silent the read
//     fails with a timeout and the handler returns as it does for any other
//     read error.
//   - Write-only handlers use StartWebSocketKeepaliveReader, which also starts
//     a reader that discards incoming messages, and select on Done.
//
// The keepalive never closes the connection on the read path; the handler must
// still `defer ws.Close()`. It closes the connection only when a ping cannot
// be written, which also unblocks a writer stuck on a dead peer.
type WebSocketKeepalive struct {
	conn *websocket.Conn
	cfg  WebSocketKeepaliveConfig

	stop     chan struct{}
	stopOnce sync.Once

	gone     chan struct{}
	goneOnce sync.Once

	// pingerDone is closed when the ping goroutine has exited.
	pingerDone chan struct{}
}

// StartWebSocketKeepalive arms keepalive for a handler that owns a read loop,
// using WebSocketKeepaliveDefaults. Call it before the read loop starts, or
// from the reader goroutine itself, because it installs the pong handler.
func StartWebSocketKeepalive(conn *websocket.Conn) *WebSocketKeepalive {
	return WebSocketKeepaliveDefaults.Start(conn)
}

// StartWebSocketKeepaliveReader arms keepalive for a handler that never reads,
// using WebSocketKeepaliveDefaults. The connection's reader belongs to the
// keepalive from then on; the handler must not read.
func StartWebSocketKeepaliveReader(conn *websocket.Conn) *WebSocketKeepalive {
	return WebSocketKeepaliveDefaults.StartReader(conn)
}

// Start arms keepalive with this timing for a handler that owns a read loop.
// See StartWebSocketKeepalive.
func (cfg WebSocketKeepaliveConfig) Start(conn *websocket.Conn) *WebSocketKeepalive {
	k := &WebSocketKeepalive{
		conn:       conn,
		cfg:        cfg,
		stop:       make(chan struct{}),
		gone:       make(chan struct{}),
		pingerDone: make(chan struct{}),
	}

	_ = k.extendReadDeadline()
	conn.SetPongHandler(func(string) error {
		return k.extendReadDeadline()
	})

	go k.pingLoop()

	return k
}

// StartReader arms keepalive with this timing for a handler that never reads.
// See StartWebSocketKeepaliveReader.
func (cfg WebSocketKeepaliveConfig) StartReader(conn *websocket.Conn) *WebSocketKeepalive {
	k := cfg.Start(conn)
	go k.discardLoop()
	return k
}

// ReadMessage pushes the read deadline forward and reads the next data
// message. Refreshing before every read means time the handler spent
// processing the previous message is never mistaken for a silent peer.
func (k *WebSocketKeepalive) ReadMessage() (messageType int, p []byte, err error) {
	_ = k.extendReadDeadline()
	return k.conn.ReadMessage()
}

// ReadJSON pushes the read deadline forward and decodes the next data message
// as JSON. See ReadMessage.
func (k *WebSocketKeepalive) ReadJSON(v any) error {
	_ = k.extendReadDeadline()
	return k.conn.ReadJSON(v)
}

// Done is closed once the peer is considered gone: a ping could not be
// written, or, for StartWebSocketKeepaliveReader, the discarding reader
// stopped because the read deadline expired, the peer closed the connection or
// the connection failed. Stop does not close it.
func (k *WebSocketKeepalive) Done() <-chan struct{} {
	return k.gone
}

// Stop tells the pinger to exit. It does not wait for an in-flight ping, does
// not close the connection and is safe to call more than once. Handlers
// should `defer keepalive.Stop()` right after starting the keepalive.
func (k *WebSocketKeepalive) Stop() {
	k.stopOnce.Do(func() {
		close(k.stop)
	})
}

func (k *WebSocketKeepalive) extendReadDeadline() error {
	return k.conn.SetReadDeadline(time.Now().Add(k.cfg.PongWait))
}

func (k *WebSocketKeepalive) markGone() {
	k.goneOnce.Do(func() {
		close(k.gone)
	})
}

func (k *WebSocketKeepalive) stopped() bool {
	select {
	case <-k.stop:
		return true
	default:
		return false
	}
}

func (k *WebSocketKeepalive) pingLoop() {
	defer close(k.pingerDone)

	ticker := time.NewTicker(k.cfg.PingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-k.stop:
			return
		case <-k.gone:
			return
		case <-ticker.C:
			err := k.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(k.cfg.WriteWait))
			if err == nil {
				continue
			}
			// The handler already sent a close frame and may be waiting for the
			// peer's reply; keep the connection open for that handshake.
			if errors.Is(err, websocket.ErrCloseSent) || k.stopped() {
				return
			}
			// The peer cannot be reached, or a writer has been stuck on it for
			// longer than WriteWait. Closing unblocks both the reader and that
			// writer so the handler releases its resources now.
			k.markGone()
			_ = k.conn.Close()
			return
		}
	}
}

func (k *WebSocketKeepalive) discardLoop() {
	defer k.markGone()

	for {
		_ = k.extendReadDeadline()
		_, reader, err := k.conn.NextReader()
		if err == nil {
			_, err = io.Copy(io.Discard, reader)
		}
		if err != nil {
			if IsUnexpectedWebsocketError(err) {
				logger.Error("WebSocket read error:", err)
			}
			return
		}
	}
}
