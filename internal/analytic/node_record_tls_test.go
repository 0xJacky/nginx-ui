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
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gorilla/websocket"
)

// newSelfSignedNodeServer serves the node HTTP probe and the analytic
// WebSocket over TLS with httptest's self-signed certificate. The WebSocket
// handler closes right after the upgrade so nodeAnalyticRecord returns promptly.
func newSelfSignedNodeServer(t *testing.T) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/node", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(NodeInfo{Version: "test"})
	})
	mux.HandleFunc("/api/analytic/intro", func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		_ = c.Close()
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func runNodeAnalyticRecordAgainst(t *testing.T, srv *httptest.Server) error {
	t.Helper()
	node := &model.Node{
		Model: model.Model{ID: 43},
		Name:  "self-signed",
		URL:   srv.URL,
	}
	setupLegacyNodeAuthForTest(t, node, "test-token")
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
	return nodeAnalyticRecord(node, ctx)
}

func setInsecureSkipVerifyForTest(t *testing.T, value bool) {
	t.Helper()
	original := settings.HTTPSettings.InsecureSkipVerify
	settings.HTTPSettings.InsecureSkipVerify = value
	t.Cleanup(func() {
		settings.HTTPSettings.InsecureSkipVerify = original
	})
}

// TestNodeAnalyticRecordHonorsInsecureSkipVerify covers a node behind a
// self-signed certificate: with [http] InsecureSkipVerify enabled, the HTTP
// probe used to pass while the WebSocket dial still failed with "x509:
// certificate signed by unknown authority", leaving the node shown offline.
func TestNodeAnalyticRecordHonorsInsecureSkipVerify(t *testing.T) {
	setInsecureSkipVerifyForTest(t, true)

	err := runNodeAnalyticRecordAgainst(t, newSelfSignedNodeServer(t))
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "connect node WebSocket") ||
		strings.Contains(err.Error(), "node HTTP probe failed") {
		t.Fatalf("expected the TLS handshake to be skipped, got %v", err)
	}
}

func TestNodeAnalyticRecordVerifiesCertificateByDefault(t *testing.T) {
	setInsecureSkipVerifyForTest(t, false)

	err := runNodeAnalyticRecordAgainst(t, newSelfSignedNodeServer(t))
	if err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("expected a certificate verification error, got %v", err)
	}
}

func TestNewNodeWebSocketDialerFollowsInsecureSkipVerify(t *testing.T) {
	for _, value := range []bool{true, false} {
		setInsecureSkipVerifyForTest(t, value)
		dial, err := newNodeWebSocketDialer()
		if err != nil {
			t.Fatalf("newNodeWebSocketDialer() error = %v", err)
		}
		if dial.TLSClientConfig == nil || dial.TLSClientConfig.InsecureSkipVerify != value {
			t.Fatalf("InsecureSkipVerify = %v, want %v", dial.TLSClientConfig, value)
		}
	}
}
