package pty

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

type chunkTerminal struct {
	chunks [][]byte
	err    error
}

func (p *chunkTerminal) Read(b []byte) (int, error) {
	if len(p.chunks) == 0 {
		return 0, p.err
	}
	n := copy(b, p.chunks[0])
	p.chunks = p.chunks[1:]
	if len(p.chunks) == 0 {
		return n, p.err
	}
	return n, nil
}
func (*chunkTerminal) Write(b []byte) (int, error) { return len(b), nil }
func (*chunkTerminal) Resize(uint16, uint16) error { return nil }
func (*chunkTerminal) Close() error                { return nil }

func TestPipelinePreservesSplitUTF8(t *testing.T) {
	text := "Nginx UI: 안녕하세요 🌍!"
	for split := 1; split < len(text); split++ {
		t.Run(fmt.Sprint(split), func(t *testing.T) {
			result := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ws, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					result <- err
					return
				}
				defer ws.Close()
				p := &Pipeline{Pty: &chunkTerminal{chunks: [][]byte{[]byte(text[:split]), []byte(text[split:])}, err: io.EOF}, ws: ws}
				p.ReadPtyAndWriteWs(result)
			}))
			defer server.Close()
			ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer ws.Close()
			ws.SetReadDeadline(time.Now().Add(3 * time.Second))
			var output strings.Builder
			for {
				_, data, err := ws.ReadMessage()
				if err != nil {
					break
				}
				if !utf8.Valid(data) {
					t.Fatalf("invalid UTF-8 frame: %q", data)
				}
				output.Write(data)
			}
			if got := output.String(); got != text {
				t.Fatalf("output %q; want %q", got, text)
			}
			if err := <-result; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPipelineNormalCompletionDoesNotHideError(t *testing.T) {
	material := errors.New("terminal device failure")
	results := make(chan error, 2)
	finished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		ws, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer ws.Close()
		p := &Pipeline{Pty: &chunkTerminal{err: material}, ws: ws}
		// Force a normal completion to occupy the first slot before the
		// other pump reports its material failure.
		p.ReadWsAndWritePty(results)
		p.ReadPtyAndWriteWs(results)
	}))
	defer server.Close()
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	if err := ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("pump completion blocked")
	}
	if err := <-results; err != nil {
		t.Fatalf("normal completion: %v", err)
	}
	if err := <-results; !errors.Is(err, material) {
		t.Fatalf("lost material error: %v", err)
	}
	select {
	case err := <-results:
		t.Fatalf("pump reported more than once: %v", err)
	default:
	}
}
