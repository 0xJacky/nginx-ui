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
	}
	setupLegacyNodeAuthForTest(t, node, "test-token")
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

func TestConnectionFailureKeepsFreshNodeOnline(t *testing.T) {
	nodeID := uint64(43)
	lastResponse := time.Now().Add(-time.Second)

	nodeMapMu.Lock()
	NodeMap[nodeID] = &Node{NodeStat: NodeStat{
		Status:     true,
		ResponseAt: lastResponse,
	}}
	nodeMapMu.Unlock()
	retryMutex.Lock()
	delete(retryStates, nodeID)
	retryMutex.Unlock()
	t.Cleanup(func() {
		nodeMapMu.Lock()
		delete(NodeMap, nodeID)
		nodeMapMu.Unlock()
		retryMutex.Lock()
		delete(retryStates, nodeID)
		retryMutex.Unlock()
	})

	markConnectionFailure(nodeID)

	nodeMapMu.RLock()
	node := cloneNode(NodeMap[nodeID])
	nodeMapMu.RUnlock()
	if !node.Status {
		t.Fatal("expected a transient connection failure to preserve fresh online status")
	}
	if !node.ResponseAt.Equal(lastResponse) {
		t.Fatalf("expected last successful response time to be preserved, got %v", node.ResponseAt)
	}
}

func TestConnectionFailureMarksStaleNodeOffline(t *testing.T) {
	nodeID := uint64(44)
	lastResponse := time.Now().Add(-nodeOfflineTimeout - time.Second)

	nodeMapMu.Lock()
	NodeMap[nodeID] = &Node{NodeStat: NodeStat{
		Status:     true,
		ResponseAt: lastResponse,
	}}
	nodeMapMu.Unlock()
	t.Cleanup(func() {
		nodeMapMu.Lock()
		delete(NodeMap, nodeID)
		nodeMapMu.Unlock()
		retryMutex.Lock()
		delete(retryStates, nodeID)
		retryMutex.Unlock()
	})

	markConnectionFailure(nodeID)

	nodeMapMu.RLock()
	node := cloneNode(NodeMap[nodeID])
	nodeMapMu.RUnlock()
	if node.Status {
		t.Fatal("expected a stale node to be marked offline")
	}
	if !node.ResponseAt.Equal(lastResponse) {
		t.Fatalf("expected offline transition to preserve last successful response time, got %v", node.ResponseAt)
	}
}

func TestSuccessfulSampleResetsRetryBackoff(t *testing.T) {
	nodeID := uint64(45)
	retryMutex.Lock()
	retryStates[nodeID] = &NodeRetryState{
		FailureCount: 7,
		NextRetry:    time.Now().Add(time.Minute),
	}
	retryMutex.Unlock()
	t.Cleanup(func() {
		retryMutex.Lock()
		delete(retryStates, nodeID)
		retryMutex.Unlock()
	})

	markConnectionSuccess(nodeID)

	retryMutex.Lock()
	state := *retryStates[nodeID]
	retryMutex.Unlock()
	if state.FailureCount != 0 {
		t.Fatalf("expected failure count to reset, got %d", state.FailureCount)
	}
	if state.NextRetry.After(time.Now()) {
		t.Fatalf("expected retry to be immediately available, got %v", state.NextRetry)
	}
}

func TestEqualNodeConfigsDetectsConnectionChanges(t *testing.T) {
	base := []*model.Node{
		{Model: model.Model{ID: 1}, Name: "node-a", URL: "https://node-a.example", EncryptedLegacySecret: []byte("token-a"), Enabled: true},
		{Model: model.Model{ID: 2}, Name: "node-b", URL: "https://node-b.example", EncryptedLegacySecret: []byte("token-b"), Enabled: true},
	}
	reordered := []*model.Node{base[1], base[0]}
	if !equalNodeConfigs(base, reordered) {
		t.Fatal("expected node ordering not to trigger a monitor reload")
	}

	changedCredential := []*model.Node{
		base[0],
		{Model: model.Model{ID: 2}, Name: "node-b", URL: "https://node-b.example", EncryptedLegacySecret: []byte("rotated-token"), Enabled: true},
	}
	if equalNodeConfigs(base, changedCredential) {
		t.Fatal("expected a credential change to trigger a monitor reload")
	}
}
