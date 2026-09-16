package sites

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
	"github.com/gorilla/websocket"
)

const keepaliveTestTimeout = 5 * time.Second

func useShortKeepalive(t *testing.T) helper.WebSocketKeepaliveConfig {
	t.Helper()

	cfg := helper.WebSocketKeepaliveConfig{
		PingPeriod: 50 * time.Millisecond,
		PongWait:   500 * time.Millisecond,
		WriteWait:  250 * time.Millisecond,
	}
	previous := helper.WebSocketKeepaliveDefaults
	helper.WebSocketKeepaliveDefaults = cfg
	t.Cleanup(func() { helper.WebSocketKeepaliveDefaults = previous })
	return cfg
}

func isManaged(conn *websocket.Conn) bool {
	wsManager.mutex.RLock()
	defer wsManager.mutex.RUnlock()
	_, ok := wsManager.connections[conn]
	return ok
}

// startSiteNavigationServer serves one site navigation session. The server
// side connection is sent on the returned channel, which is closed once the
// session has ended.
func startSiteNavigationServer(t *testing.T) (string, <-chan *websocket.Conn) {
	t.Helper()

	service := sitecheck.NewService(context.Background(), sitecheck.CheckOptions{})
	sessions := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(sessions)
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		sessions <- conn
		serveSiteNavigation(conn, service)
	}))
	t.Cleanup(server.Close)

	return "ws" + strings.TrimPrefix(server.URL, "http"), sessions
}

// dialSiteNavigation connects and reads the initial message itself, then keeps
// reading in the background, forwarding data messages and ping notifications.
func dialSiteNavigation(t *testing.T, url string, answerPings bool) (*websocket.Conn, <-chan ServerMessage, <-chan struct{}) {
	t.Helper()

	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Close() })

	pings := make(chan struct{}, 1024)
	ws.SetPingHandler(func(data string) error {
		select {
		case pings <- struct{}{}:
		default:
		}
		if !answerPings {
			return nil
		}
		return ws.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
	})

	var initial ServerMessage
	if err := ws.ReadJSON(&initial); err != nil {
		t.Fatal(err)
	}
	if initial.Type != MessageTypeInitial {
		t.Fatalf("first message type %q; want %q", initial.Type, MessageTypeInitial)
	}

	messages := make(chan ServerMessage, 16)
	go func() {
		for {
			var message ServerMessage
			if err := ws.ReadJSON(&message); err != nil {
				return
			}
			messages <- message
		}
	}()

	return ws, messages, pings
}

func waitForSessionEnd(t *testing.T, sessions <-chan *websocket.Conn, reason string) {
	t.Helper()

	select {
	case _, ok := <-sessions:
		if ok {
			t.Fatal("unexpected second session")
		}
	case <-time.After(keepaliveTestTimeout):
		t.Fatal(reason)
	}
}

func TestSiteNavigationWebSocketDropsPeerThatStopsAnsweringPings(t *testing.T) {
	useShortKeepalive(t)
	url, sessions := startSiteNavigationServer(t)
	dialSiteNavigation(t, url, false)

	conn := <-sessions
	if !isManaged(conn) {
		t.Fatal("session was not registered for broadcasts")
	}

	waitForSessionEnd(t, sessions, "site navigation session outlived a peer that stopped answering pings")
	if isManaged(conn) {
		t.Fatal("dead peer is still registered for broadcasts")
	}
}

func TestSiteNavigationWebSocketKeepsResponsivePeer(t *testing.T) {
	cfg := useShortKeepalive(t)
	url, sessions := startSiteNavigationServer(t)
	ws, messages, pings := dialSiteNavigation(t, url, true)
	conn := <-sessions

	idlePings := int(2*cfg.PongWait/cfg.PingPeriod) + 1
	timeout := time.After(keepaliveTestTimeout)
	for i := 0; i < idlePings; i++ {
		select {
		case <-pings:
		case <-sessions:
			t.Fatalf("session ended after %d of %d pings", i, idlePings)
		case <-timeout:
			t.Fatalf("saw %d of %d pings", i, idlePings)
		}
	}
	if !isManaged(conn) {
		t.Fatal("live peer was removed from broadcasts")
	}

	// Application messages still flow through the keepalive read loop.
	if err := ws.WriteJSON(ClientMessage{Type: MessageTypePing}); err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-messages:
		if message.Type != MessageTypePong {
			t.Fatalf("reply type %q; want %q", message.Type, MessageTypePong)
		}
	case <-time.After(keepaliveTestTimeout):
		t.Fatal("no pong reply")
	}

	message := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	if err := ws.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	waitForSessionEnd(t, sessions, "site navigation session did not end after a normal close")
	if isManaged(conn) {
		t.Fatal("closed peer is still registered for broadcasts")
	}
}
