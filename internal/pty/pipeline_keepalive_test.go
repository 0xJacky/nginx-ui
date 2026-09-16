package pty

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/gorilla/websocket"
)

const keepaliveTestTimeout = 5 * time.Second

// idleTerminal models a shell waiting for input: Read blocks until Close, as
// a real PTY does once its process is killed.
type idleTerminal struct {
	closed    chan struct{}
	closeOnce sync.Once
	input     chan string
}

func newIdleTerminal() *idleTerminal {
	return &idleTerminal{
		closed: make(chan struct{}),
		input:  make(chan string, 16),
	}
}

func (p *idleTerminal) Read([]byte) (int, error) {
	<-p.closed
	return 0, io.EOF
}

func (p *idleTerminal) Write(b []byte) (int, error) {
	p.input <- string(b)
	return len(b), nil
}

func (*idleTerminal) Resize(uint16, uint16) error { return nil }

func (p *idleTerminal) Close() error {
	p.closeOnce.Do(func() { close(p.closed) })
	return nil
}

// useShortKeepalive shortens the shared keepalive timing for one test.
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

// startTerminalSession serves one pipeline the way api/terminal.Pty does and
// closes finished once both pumps have reported.
func startTerminalSession(t *testing.T, terminal *idleTerminal) (string, <-chan struct{}) {
	t.Helper()

	finished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		ws, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer ws.Close()

		p := &Pipeline{Pty: terminal, ws: ws}
		defer p.Close()

		errorChan := make(chan error, 2)
		go p.ReadPtyAndWriteWs(errorChan)
		go p.ReadWsAndWritePty(errorChan)

		if err := <-errorChan; err != nil {
			t.Error(err)
		}
		p.Close()
		if err := <-errorChan; err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)

	return "ws" + strings.TrimPrefix(server.URL, "http"), finished
}

// dialTerminal connects and keeps reading so control frames are processed.
// Every ping the client sees is reported on the returned channel.
func dialTerminal(t *testing.T, url string, answerPings bool) (*websocket.Conn, <-chan struct{}) {
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
	go func() {
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	}()

	return ws, pings
}

func TestPipelineClosesTerminalWhenPeerStopsAnsweringPings(t *testing.T) {
	useShortKeepalive(t)
	terminal := newIdleTerminal()
	url, finished := startTerminalSession(t, terminal)
	dialTerminal(t, url, false)

	select {
	case <-finished:
	case <-time.After(keepaliveTestTimeout):
		t.Fatal("terminal session outlived a peer that stopped answering pings")
	}
	select {
	case <-terminal.closed:
	default:
		t.Fatal("terminal was not closed, so its shell would keep running")
	}
}

func TestPipelineKeepsIdleResponsiveSession(t *testing.T) {
	cfg := useShortKeepalive(t)
	terminal := newIdleTerminal()
	url, finished := startTerminalSession(t, terminal)
	ws, pings := dialTerminal(t, url, true)

	// Stay idle for twice the pong wait; only pongs keep the session alive.
	idlePings := int(2*cfg.PongWait/cfg.PingPeriod) + 1
	timeout := time.After(keepaliveTestTimeout)
	for i := 0; i < idlePings; i++ {
		select {
		case <-pings:
		case <-finished:
			t.Fatalf("idle session closed after %d of %d pings", i, idlePings)
		case <-timeout:
			t.Fatalf("saw %d of %d pings", i, idlePings)
		}
	}

	// The frontend's application-level ping and regular input still flow.
	if err := ws.WriteJSON(map[string]any{"Type": TypePing, "Data": nil}); err != nil {
		t.Fatal(err)
	}
	if err := ws.WriteJSON(map[string]any{"Type": TypeData, "Data": "uptime\r"}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-terminal.input:
		if got != "uptime\r" {
			t.Fatalf("terminal input %q; want %q", got, "uptime\r")
		}
	case <-time.After(keepaliveTestTimeout):
		t.Fatal("input did not reach the terminal")
	}
	select {
	case <-terminal.closed:
		t.Fatal("terminal closed while the browser was alive")
	default:
	}

	message := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	if err := ws.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
	case <-time.After(keepaliveTestTimeout):
		t.Fatal("session did not end after a normal close")
	}
}
