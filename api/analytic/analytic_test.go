package analytic

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestAnalyticWebSocketCPUUsesNormalizedSample(t *testing.T) {
	testAnalyticWebSocketCPU(t, false)
}

func TestAnalyticWebSocketCPURecoversFromSampleError(t *testing.T) {
	testAnalyticWebSocketCPU(t, true)
}

func testAnalyticWebSocketCPU(t *testing.T, failFirst bool) {
	t.Helper()
	previous := kernel.Context
	ctx, cancel := context.WithCancel(context.Background())
	kernel.Context = ctx
	defer func() { kernel.Context = previous }()
	defer cancel()

	done := make(chan struct{})
	router := gin.New()
	router.GET("/analytic", func(c *gin.Context) {
		defer close(done)
		serveAnalytic(c, func() (float64, float64, error) {
			if failFirst {
				failFirst = false
				return 0, 0, errors.New("CPU sample unavailable")
			}
			return 12.5, 25, nil
		})
	})
	server := httptest.NewServer(router)
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/analytic", http.Header{"Origin": []string{server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		conn.Close()
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("analytics handler did not stop")
		}
	}()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var message Stat
	if err := conn.ReadJSON(&message); err != nil {
		t.Fatal(err)
	}
	want := CPUStat{User: 12.5, System: 12.5, Idle: 75, Total: 25}
	if message.CPU != want {
		t.Fatalf("WebSocket CPU = %+v; want %+v", message.CPU, want)
	}
	// Existing dashboard clients derive the live total from user + system.
	if message.CPU.User+message.CPU.System != message.CPU.Total {
		t.Fatal("live chart sum differs from total")
	}
}
