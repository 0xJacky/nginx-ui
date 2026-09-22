package helper

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Short enough to keep the suite fast, with enough slack between PingPeriod
// and PongWait that a busy -race runner does not drop a live peer.
var testKeepaliveConfig = WebSocketKeepaliveConfig{
	PingPeriod: 50 * time.Millisecond,
	PongWait:   500 * time.Millisecond,
	WriteWait:  250 * time.Millisecond,
}

// keepaliveExitTimeout is far below the production 60s pong wait, so passing
// proves the shortened keepalive, not TCP, ended the session.
const keepaliveExitTimeout = 5 * time.Second

// pingsCoveringPongWait is enough ping periods to outlast PongWait twice over.
var pingsCoveringPongWait = int(2*testKeepaliveConfig.PongWait/testKeepaliveConfig.PingPeriod) + 1

// startKeepaliveServer runs handler for each WebSocket session and returns the
// ws:// URL. Its returned channel receives handler's result once it returns.
func startKeepaliveServer(t *testing.T, handler func(ws *websocket.Conn) error) (string, <-chan error) {
	t.Helper()

	results := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			results <- err
			return
		}
		defer ws.Close()
		results <- handler(ws)
	}))
	t.Cleanup(server.Close)

	return "ws" + strings.TrimPrefix(server.URL, "http"), results
}

// keepaliveClient reads continuously so control frames are processed, and
// forwards data messages and ping notifications to channels.
type keepaliveClient struct {
	conn     *websocket.Conn
	pings    chan struct{}
	messages chan []byte
	closed   chan struct{}
}

// dialKeepaliveClient connects to url. A client that does not answer pings
// still reads, so the server sees a TCP-alive peer that only went silent.
func dialKeepaliveClient(t *testing.T, url string, answerPings bool) *keepaliveClient {
	t.Helper()

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := &keepaliveClient{
		conn:     conn,
		pings:    make(chan struct{}, 1024),
		messages: make(chan []byte, 16),
		closed:   make(chan struct{}),
	}

	conn.SetPingHandler(func(data string) error {
		select {
		case client.pings <- struct{}{}:
		default:
		}
		if !answerPings {
			return nil
		}
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
	})

	go func() {
		defer close(client.closed)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			client.messages <- data
		}
	}()

	return client
}

// waitForPings blocks until the client has seen count pings.
func (c *keepaliveClient) waitForPings(t *testing.T, count int) {
	t.Helper()

	timeout := time.After(keepaliveExitTimeout + time.Duration(count)*testKeepaliveConfig.PingPeriod)
	for i := 0; i < count; i++ {
		select {
		case <-c.pings:
		case <-c.closed:
			t.Fatalf("connection closed after %d of %d pings", i, count)
		case <-timeout:
			t.Fatalf("saw %d of %d pings", i, count)
		}
	}
}

func (c *keepaliveClient) expectMessage(t *testing.T, want string) {
	t.Helper()

	select {
	case got := <-c.messages:
		if string(got) != want {
			t.Fatalf("message %q; want %q", got, want)
		}
	case <-c.closed:
		t.Fatalf("connection closed while waiting for %q", want)
	case <-time.After(keepaliveExitTimeout):
		t.Fatalf("timed out waiting for %q", want)
	}
}

func (c *keepaliveClient) closeNormally(t *testing.T) {
	t.Helper()

	message := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	if err := c.conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
}

func waitForResult(t *testing.T, results <-chan error) error {
	t.Helper()

	select {
	case err := <-results:
		return err
	case <-time.After(keepaliveExitTimeout):
		t.Fatal("handler did not return")
		return nil
	}
}

func requireNoResult(t *testing.T, results <-chan error) {
	t.Helper()

	select {
	case err := <-results:
		t.Fatalf("handler returned while the peer was alive: %v", err)
	default:
	}
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// echoUntilError is a read-loop handler shape: it echoes every data message.
func echoUntilError(ws *websocket.Conn) error {
	keepalive := testKeepaliveConfig.Start(ws)
	defer keepalive.Stop()

	for {
		messageType, data, err := keepalive.ReadMessage()
		if err != nil {
			return err
		}
		_ = ws.SetWriteDeadline(time.Now().Add(testKeepaliveConfig.WriteWait))
		if err := ws.WriteMessage(messageType, data); err != nil {
			return err
		}
	}
}

// waitForPeerGone is a write-only handler shape.
func waitForPeerGone(ws *websocket.Conn) error {
	keepalive := testKeepaliveConfig.StartReader(ws)
	defer keepalive.Stop()

	<-keepalive.Done()
	return nil
}

func TestWebSocketKeepaliveReadLoopDropsSilentPeer(t *testing.T) {
	url, results := startKeepaliveServer(t, echoUntilError)
	client := dialKeepaliveClient(t, url, false)

	started := time.Now()
	err := waitForResult(t, results)
	if !isTimeout(err) {
		t.Fatalf("read loop ended with %v; want a read deadline timeout", err)
	}
	if elapsed := time.Since(started); elapsed < testKeepaliveConfig.PongWait/2 {
		t.Fatalf("read loop ended after %v, before the pong wait could expire", elapsed)
	}
	if len(client.pings) == 0 {
		t.Fatal("server never pinged the peer")
	}
}

func TestWebSocketKeepaliveReaderDropsSilentPeer(t *testing.T) {
	url, results := startKeepaliveServer(t, waitForPeerGone)
	client := dialKeepaliveClient(t, url, false)

	if err := waitForResult(t, results); err != nil {
		t.Fatal(err)
	}
	// The handler's deferred Close must reach the client.
	select {
	case <-client.closed:
	case <-time.After(keepaliveExitTimeout):
		t.Fatal("client connection was not closed")
	}
}

func TestWebSocketKeepaliveKeepsResponsivePeer(t *testing.T) {
	for name, handler := range map[string]func(*websocket.Conn) error{
		"read loop":  echoUntilError,
		"write only": waitForPeerGone,
	} {
		t.Run(name, func(t *testing.T) {
			url, results := startKeepaliveServer(t, handler)
			client := dialKeepaliveClient(t, url, true)

			client.waitForPings(t, pingsCoveringPongWait)
			requireNoResult(t, results)

			client.closeNormally(t)
			if err := waitForResult(t, results); isTimeout(err) {
				t.Fatalf("handler timed out instead of seeing the close frame: %v", err)
			}
		})
	}
}

func TestWebSocketKeepaliveReadLoopDeliversMessages(t *testing.T) {
	url, results := startKeepaliveServer(t, echoUntilError)
	client := dialKeepaliveClient(t, url, true)

	if err := client.conn.WriteMessage(websocket.TextMessage, []byte("first")); err != nil {
		t.Fatal(err)
	}
	client.expectMessage(t, "first")

	// Stay idle past the pong wait; only pongs keep the session alive here.
	client.waitForPings(t, pingsCoveringPongWait)

	if err := client.conn.WriteMessage(websocket.TextMessage, []byte("second")); err != nil {
		t.Fatal(err)
	}
	client.expectMessage(t, "second")
	requireNoResult(t, results)

	client.closeNormally(t)
	err := waitForResult(t, results)
	if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		t.Fatalf("read loop ended with %v; want a normal close", err)
	}
}

func TestWebSocketKeepaliveSlowMessageHandlingIsNotSilence(t *testing.T) {
	release := make(chan struct{})
	url, results := startKeepaliveServer(t, func(ws *websocket.Conn) error {
		keepalive := testKeepaliveConfig.Start(ws)
		defer keepalive.Stop()

		if _, _, err := keepalive.ReadMessage(); err != nil {
			return err
		}
		// Model a handler blocked on the message, such as a PTY write, for
		// longer than the pong wait while the peer keeps answering pings.
		<-release

		_, data, err := keepalive.ReadMessage()
		if err != nil {
			return err
		}
		if string(data) != "after" {
			return errors.New("unexpected message " + string(data))
		}
		return nil
	})
	client := dialKeepaliveClient(t, url, true)

	if err := client.conn.WriteMessage(websocket.TextMessage, []byte("before")); err != nil {
		t.Fatal(err)
	}
	client.waitForPings(t, pingsCoveringPongWait)
	if err := client.conn.WriteMessage(websocket.TextMessage, []byte("after")); err != nil {
		t.Fatal(err)
	}
	close(release)

	if err := waitForResult(t, results); err != nil {
		t.Fatalf("slow handler lost a live peer: %v", err)
	}
}

func TestWebSocketKeepalivePingFailureClosesConnection(t *testing.T) {
	gone := make(chan *WebSocketKeepalive, 1)
	url, results := startKeepaliveServer(t, func(ws *websocket.Conn) error {
		keepalive := testKeepaliveConfig.Start(ws)
		defer keepalive.Stop()

		// No reader runs, so only a failed ping can signal Done.
		_ = ws.NetConn().Close()
		select {
		case <-keepalive.Done():
			gone <- keepalive
			return nil
		case <-time.After(keepaliveExitTimeout):
			return errors.New("ping failure did not signal Done")
		}
	})
	dialKeepaliveClient(t, url, true)

	if err := waitForResult(t, results); err != nil {
		t.Fatal(err)
	}
	keepalive := <-gone
	select {
	case <-keepalive.pingerDone:
	case <-time.After(keepaliveExitTimeout):
		t.Fatal("pinger did not exit after a failed ping")
	}
}

func TestWebSocketKeepaliveKeepsCloseHandshakeOpen(t *testing.T) {
	url, results := startKeepaliveServer(t, func(ws *websocket.Conn) error {
		keepalive := testKeepaliveConfig.Start(ws)
		defer keepalive.Stop()

		message := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		if err := ws.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second)); err != nil {
			return err
		}
		select {
		case <-keepalive.pingerDone:
		case <-time.After(keepaliveExitTimeout):
			return errors.New("pinger kept running after the close frame")
		}
		select {
		case <-keepalive.Done():
			return errors.New("sending a close frame was treated as a lost peer")
		default:
		}
		// The connection must still be open to receive the peer's reply.
		_, _, err := keepalive.ReadMessage()
		return err
	})
	dialKeepaliveClient(t, url, true)

	err := waitForResult(t, results)
	if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		t.Fatalf("close handshake ended with %v; want the peer's normal close", err)
	}
}

func TestWebSocketKeepaliveStopEndsPinger(t *testing.T) {
	url, results := startKeepaliveServer(t, func(ws *websocket.Conn) error {
		keepalive := testKeepaliveConfig.Start(ws)
		keepalive.Stop()
		keepalive.Stop()

		select {
		case <-keepalive.pingerDone:
		case <-time.After(keepaliveExitTimeout):
			return errors.New("pinger kept running after Stop")
		}
		select {
		case <-keepalive.Done():
			return errors.New("Stop signalled a lost peer")
		default:
		}
		return nil
	})
	dialKeepaliveClient(t, url, true)

	if err := waitForResult(t, results); err != nil {
		t.Fatal(err)
	}
}
