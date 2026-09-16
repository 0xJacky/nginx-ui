package llm

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/settings"
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

// startCodeCompletionServer serves CodeCompletion with completion enabled and
// closes the returned channel once the handler has returned.
func startCodeCompletionServer(t *testing.T) (string, http.Header, <-chan struct{}) {
	t.Helper()

	previous := settings.OpenAISettings.EnableCodeCompletion
	settings.OpenAISettings.EnableCodeCompletion = true
	t.Cleanup(func() { settings.OpenAISettings.EnableCodeCompletion = previous })

	gin.SetMode(gin.TestMode)
	exited := make(chan struct{})
	router := gin.New()
	router.GET("/code_completion", func(c *gin.Context) {
		defer close(exited)
		CodeCompletion(c)
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/code_completion"
	return url, http.Header{"Origin": {server.URL}}, exited
}

func dialCodeCompletion(t *testing.T, url string, header http.Header, answerPings bool) (*websocket.Conn, <-chan struct{}) {
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

func TestCodeCompletionReleasesPeerThatStopsAnsweringPings(t *testing.T) {
	useShortKeepalive(t)
	url, header, exited := startCodeCompletionServer(t)
	dialCodeCompletion(t, url, header, false)

	select {
	case <-exited:
	case <-time.After(keepaliveTestTimeout):
		t.Fatal("code completion handler stayed blocked on a peer that stopped answering pings")
	}
}

func TestCodeCompletionKeepsIdleResponsivePeer(t *testing.T) {
	cfg := useShortKeepalive(t)
	url, header, exited := startCodeCompletionServer(t)
	ws, pings := dialCodeCompletion(t, url, header, true)

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
		t.Fatal("code completion handler did not exit after a normal close")
	}
}
