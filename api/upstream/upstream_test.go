package upstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
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

func useKernelContext(t *testing.T) {
	t.Helper()

	previous := kernel.Context
	kernel.Context = context.Background()
	t.Cleanup(func() { kernel.Context = previous })
}

func connectionCount() int {
	wsConnectionMutex.Lock()
	defer wsConnectionMutex.Unlock()
	return wsConnections
}

// startAvailabilityServer serves AvailabilityWebSocket and closes the returned
// channel once the handler has returned.
func startAvailabilityServer(t *testing.T) (*httptest.Server, <-chan struct{}) {
	t.Helper()

	exited := make(chan struct{})
	router := gin.New()
	router.GET("/availability_ws", func(c *gin.Context) {
		defer close(exited)
		AvailabilityWebSocket(c)
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	return server, exited
}

// dialAvailability connects as a same-origin browser would. The first pushed
// snapshot is read before returning, which proves the connection registered.
func dialAvailability(t *testing.T, server *httptest.Server, answerPings bool) (*websocket.Conn, <-chan struct{}) {
	t.Helper()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/availability_ws"
	ws, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Origin": {server.URL}})
	require.NoError(t, err)
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

	var initial map[string]any
	require.NoError(t, ws.ReadJSON(&initial))

	go func() {
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	}()

	return ws, pings
}

func TestAvailabilityWebSocketReleasesPeerThatStopsAnsweringPings(t *testing.T) {
	useShortKeepalive(t)
	useKernelContext(t)
	require.Zero(t, connectionCount())

	server, exited := startAvailabilityServer(t)
	dialAvailability(t, server, false)
	require.Equal(t, 1, connectionCount())

	select {
	case <-exited:
	case <-time.After(keepaliveTestTimeout):
		t.Fatal("availability handler outlived a peer that stopped answering pings")
	}
	require.Zero(t, connectionCount(), "a dead peer must not keep the check frequency raised")
}

func TestAvailabilityWebSocketKeepsResponsivePeer(t *testing.T) {
	cfg := useShortKeepalive(t)
	useKernelContext(t)
	require.Zero(t, connectionCount())

	server, exited := startAvailabilityServer(t)
	ws, pings := dialAvailability(t, server, true)

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
	require.Equal(t, 1, connectionCount())

	message := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	require.NoError(t, ws.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second)))
	select {
	case <-exited:
	case <-time.After(keepaliveTestTimeout):
		t.Fatal("availability handler did not exit after a normal close")
	}
	require.Zero(t, connectionCount())
}
