package nginx_log

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/gin-gonic/gin"
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

// startLogServer serves Log and closes the returned channel once the handler
// has returned.
func startLogServer(t *testing.T) (string, http.Header, <-chan struct{}) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	exited := make(chan struct{})
	router := gin.New()
	router.GET("/nginx_log", func(c *gin.Context) {
		defer close(exited)
		Log(c)
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/nginx_log"
	return url, http.Header{"Origin": {server.URL}}, exited
}

func dialLog(t *testing.T, url string, header http.Header, answerPings bool) (*websocket.Conn, <-chan struct{}) {
	t.Helper()

	ws, _, err := websocket.DefaultDialer.Dial(url, header)
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
	go func() {
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	}()

	return ws, pings
}

func TestLogWebSocketReleasesPeerThatStopsAnsweringPings(t *testing.T) {
	useShortKeepalive(t)
	url, header, exited := startLogServer(t)
	dialLog(t, url, header, false)

	select {
	case <-exited:
	case <-time.After(keepaliveTestTimeout):
		t.Fatal("log handler outlived a peer that stopped answering pings")
	}
}

func TestLogWebSocketKeepsIdleResponsivePeer(t *testing.T) {
	cfg := useShortKeepalive(t)
	url, header, exited := startLogServer(t)
	ws, pings := dialLog(t, url, header, true)

	idlePings := int(2*cfg.PongWait/cfg.PingPeriod) + 1
	timeout := time.After(keepaliveTestTimeout)
	for i := 0; i < idlePings; i++ {
		select {
		case <-pings:
		case <-exited:
			t.Fatalf("handler exited after %d of %d pings", i, idlePings)
		case <-timeout:
			t.Fatalf("saw %d of %d pings", i, idlePings)
		}
	}

	message := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	if err := ws.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(keepaliveTestTimeout):
		t.Fatal("log handler did not exit after a normal close")
	}
}

func TestTailNginxLogStopsWithSession(t *testing.T) {
	done := make(chan struct{})
	returned := make(chan struct{})
	errChan := make(chan error, 2)

	// No control message ever arrives, as when the peer vanishes before
	// choosing a log. The tail goroutine must not wait for one forever.
	go func() {
		defer close(returned)
		tailNginxLog(done, nil, make(chan controlStruct), errChan)
	}()

	close(done)
	select {
	case <-returned:
	case <-time.After(keepaliveTestTimeout):
		t.Fatal("tail goroutine leaked after the session ended")
	}
}

func TestHandleLogControlStopsWithSession(t *testing.T) {
	useShortKeepalive(t)

	done := make(chan struct{})
	returned := make(chan struct{})
	errChan := make(chan error, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(returned)
		ws, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer ws.Close()

		keepalive := helper.StartWebSocketKeepalive(ws)
		defer keepalive.Stop()
		// Nobody drains controlChan, as when the tail goroutine already quit.
		handleLogControl(done, keepalive, make(chan controlStruct), errChan)
	}))
	t.Cleanup(server.Close)

	ws, _ := dialLog(t, "ws"+strings.TrimPrefix(server.URL, "http"), nil, true)
	if err := ws.WriteJSON(controlStruct{Type: "access"}); err != nil {
		t.Fatal(err)
	}

	close(done)
	select {
	case <-returned:
	case <-time.After(keepaliveTestTimeout):
		t.Fatal("control goroutine stayed blocked on a control nobody reads")
	}
	select {
	case err := <-errChan:
		t.Fatalf("session end reported %v", err)
	default:
	}
}
