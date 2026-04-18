package analytic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gorilla/websocket"
)

// TestNodeAnalyticRecordHalfDeadConnection reproduces the bug that caused node
// status to freeze until the nginx-ui process was restarted: a remote node
// that accepts the WebSocket upgrade but then stops responding (e.g. silent
// TCP hang, peer frozen) used to leave nodeAnalyticRecord blocked on ReadJSON
// forever, starving the per-node retry loop. With the keepalive in place,
// ReadJSON must unblock within pongWait and return an error so the caller can
// schedule a reconnect.
func TestNodeAnalyticRecordHalfDeadConnection(t *testing.T) {
	// Shrink the keepalive window so the test finishes quickly. Restore on exit
	// so other tests in the package see the production values.
	origPong, origPing, origWrite := nodeWSPongWait, nodeWSPingPeriod, nodeWSWriteWait
	nodeWSPongWait = 300 * time.Millisecond
	nodeWSPingPeriod = 100 * time.Millisecond
	nodeWSWriteWait = 100 * time.Millisecond
	t.Cleanup(func() {
		nodeWSPongWait, nodeWSPingPeriod, nodeWSWriteWait = origPong, origPing, origWrite
	})

	// A test server that satisfies InitNode's HTTP probe and then accepts the
	// analytic WebSocket upgrade but never writes a message or answers a ping.
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/node", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(NodeInfo{Version: "test"})
	})
	mux.HandleFunc("/api/analytic/intro", func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		// Swallow the ping by overriding the default pong-on-ping handler: do
		// nothing, so the client's read deadline must expire on its own.
		c.SetPingHandler(func(string) error { return nil })
		// Block until the connection is closed by the peer.
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// Use the raw httptest URL; GetWebSocketURL will rewrite http:// to ws://.
	node := &model.Node{
		Model: model.Model{ID: 42},
		Name:  "half-dead",
		URL:   srv.URL,
		Token: "test-token",
	}
	// Make sure the NodeMap slot exists so updateNodeStatus is a no-op on the
	// shared map across parallel tests.
	nodeMapMu.Lock()
	if NodeMap == nil {
		NodeMap = make(TNodeMap)
	}
	nodeMapMu.Unlock()
	t.Cleanup(func() {
		nodeMapMu.Lock()
		delete(NodeMap, node.ID)
		nodeMapMu.Unlock()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- nodeAnalyticRecord(node, ctx)
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatalf("expected nodeAnalyticRecord to fail on read deadline, got nil")
		}
		// Read-deadline expiry surfaces as an i/o timeout wrapped in the
		// websocket close-error path; either way it must be non-nil.
		if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "closed") {
			t.Logf("returned err = %v (non-nil, acceptable)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("nodeAnalyticRecord did not return within 2s — read deadline / ping-pong not enforced")
	}
}
