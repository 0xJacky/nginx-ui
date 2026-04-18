package analytic

import (
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

// Analytic push handlers only write to the client — they never read. Without
// ping/pong and read deadlines, a silently half-closed TCP connection keeps
// the server looping and writing for hours until the OS finally surfaces the
// error, while the client sees no updates and never triggers auto-reconnect.
// These constants mirror the values used by api/cluster/websocket.go so both
// sides of the cluster share the same keepalive contract.
const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = (wsPongWait * 9) / 10
)

// startWSKeepalive arms a read deadline + pong handler on the connection and
// spawns two goroutines: a reader that drains control frames (so pongs reset
// the deadline) and a pinger that emits a ping on pingPeriod. When the peer
// stops responding, the read deadline fires, the reader returns, and done is
// closed. The handler's deferred ws.Close() ultimately releases the socket —
// this helper never calls Close() itself on the read path, so a caller must
// `defer ws.Close()` after upgrade.
//
// The returned done channel is closed once the reader exits (read error,
// read deadline expiry, or peer close). Callers should select on it to bail
// out of their write loop promptly instead of waiting for the next WriteJSON
// to fail.
func startWSKeepalive(ws *websocket.Conn) <-chan struct{} {
	done := make(chan struct{})

	_ = ws.SetReadDeadline(time.Now().Add(wsPongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	go func() {
		defer close(done)
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				if helper.IsUnexpectedWebsocketError(err) {
					logger.Error("WebSocket read error:", err)
				}
				return
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(wsPingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsWriteWait)); err != nil {
					_ = ws.Close()
					return
				}
			}
		}
	}()

	return done
}
