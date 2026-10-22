package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pipePair wires two Conns back to back through in-memory pipes and serves
// both of them for the lifetime of the test.
func pipePair(t *testing.T, opts ...Option) (*Conn, *Conn) {
	t.Helper()

	aReader, bWriter := io.Pipe()
	bReader, aWriter := io.Pipe()

	a := NewConn(aReader, aWriter, opts...)
	b := NewConn(bReader, bWriter, opts...)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = a.Serve(ctx) }()
	go func() { defer wg.Done(); _ = b.Serve(ctx) }()

	t.Cleanup(func() {
		cancel()
		_ = a.Close()
		_ = b.Close()
		wg.Wait()
	})

	return a, b
}

type echoParams struct {
	Text string `json:"text"`
}

type echoResult struct {
	Text string `json:"text"`
}

func TestCallRequestResponse(t *testing.T) {
	client, server := pipePair(t)

	server.Handle("echo", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p echoParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, Errorf(protocol.CodeInvalidParams, err.Error())
		}
		return echoResult{Text: strings.ToUpper(p.Text)}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var res echoResult
	require.NoError(t, client.Call(ctx, "echo", echoParams{Text: "ping"}, &res))
	assert.Equal(t, "PING", res.Text)

	// A nil result discards the payload without failing.
	require.NoError(t, client.Call(ctx, "echo", echoParams{Text: "ping"}, nil))
}

func TestCallConcurrentInFlight(t *testing.T) {
	client, server := pipePair(t)

	server.Handle("slow.echo", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p echoParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		// Stagger replies so ids cannot be matched by arrival order.
		time.Sleep(time.Duration(len(p.Text)%7) * time.Millisecond)
		return echoResult{Text: p.Text}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	const calls = 100
	var wg sync.WaitGroup
	errs := make(chan error, calls)
	for i := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			want := fmt.Sprintf("payload-%03d", i)
			var res echoResult
			if err := client.Call(ctx, "slow.echo", echoParams{Text: want}, &res); err != nil {
				errs <- err
				return
			}
			if res.Text != want {
				errs <- fmt.Errorf("got %q, want %q", res.Text, want)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestNotifyIsNotAnswered(t *testing.T) {
	client, server := pipePair(t)

	received := make(chan string, 1)
	server.Handle("events.on", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p echoParams
		_ = json.Unmarshal(params, &p)
		received <- p.Text
		// A notification handler may return a value, it must never be sent.
		return echoResult{Text: "ignored"}, nil
	})

	// Any response would desynchronise the client, so assert it never arrives
	// by making a later call succeed with the expected id.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, client.Notify(ctx, "events.on", echoParams{Text: "hello"}))
	select {
	case got := <-received:
		assert.Equal(t, "hello", got)
	case <-time.After(5 * time.Second):
		t.Fatal("notification was not delivered")
	}

	server.Handle("echo", func(ctx context.Context, params json.RawMessage) (any, error) {
		return echoResult{Text: "ok"}, nil
	})
	var res echoResult
	require.NoError(t, client.Call(ctx, "echo", nil, &res))
	assert.Equal(t, "ok", res.Text)
}

func TestNotifyBatch(t *testing.T) {
	client, server := pipePair(t)

	received := make(chan string, 3)
	server.Handle("events.on", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p echoParams
		_ = json.Unmarshal(params, &p)
		received <- p.Text
		return nil, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, client.NotifyBatch(ctx, []Notification{
		{Method: "events.on", Params: echoParams{Text: "a"}},
		{Method: "events.on", Params: echoParams{Text: "b"}},
		{Method: "events.on", Params: echoParams{Text: "c"}},
	}))

	got := make([]string, 0, 3)
	for range 3 {
		select {
		case v := <-received:
			got = append(got, v)
		case <-time.After(5 * time.Second):
			t.Fatal("batch notification was not delivered")
		}
	}
	assert.ElementsMatch(t, []string{"a", "b", "c"}, got)

	assert.Error(t, client.NotifyBatch(ctx, []Notification{{Params: echoParams{Text: "d"}}}))
	assert.NoError(t, client.NotifyBatch(ctx, nil))
}

func TestBatchRequestInput(t *testing.T) {
	// The peer under test only reads, so a raw writer is enough to feed it a
	// batch frame and collect the individual replies.
	serverIn, clientOut := io.Pipe()
	clientIn, serverOut := io.Pipe()

	server := NewConn(serverIn, serverOut)
	server.Handle("echo", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p echoParams
		_ = json.Unmarshal(params, &p)
		return echoResult{Text: p.Text}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = server.Serve(ctx) }()
	t.Cleanup(func() { _ = server.Close() })

	batch := `[{"jsonrpc":"2.0","id":1,"method":"echo","params":{"text":"one"}},` +
		`{"jsonrpc":"2.0","method":"echo","params":{"text":"note"}},` +
		`{"jsonrpc":"2.0","id":2,"method":"echo","params":{"text":"two"}}]` + "\n"
	go func() { _, _ = clientOut.Write([]byte(batch)) }()

	decoder := json.NewDecoder(clientIn)
	seen := map[float64]string{}
	for range 2 {
		var resp struct {
			ID     float64         `json:"id"`
			Result echoResult      `json:"result"`
			Error  *protocol.Error `json:"error"`
		}
		require.NoError(t, decoder.Decode(&resp))
		require.Nil(t, resp.Error)
		seen[resp.ID] = resp.Result.Text
	}
	assert.Equal(t, map[float64]string{1: "one", 2: "two"}, seen)
}

func TestMethodNotFound(t *testing.T) {
	client, _ := pipePair(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Call(ctx, "does.not.exist", nil, nil)
	require.Error(t, err)
	assert.True(t, IsMethodNotFound(err))
	assert.False(t, IsUnsupported(err))

	perr, ok := AsProtocolError(err)
	require.True(t, ok)
	assert.Equal(t, protocol.CodeMethodNotFound, perr.Code)
}

func TestHandlerErrorsArePassedThrough(t *testing.T) {
	client, server := pipePair(t)

	server.Handle("needs.params", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p echoParams
		if err := json.Unmarshal(params, &p); err != nil || p.Text == "" {
			return nil, &protocol.Error{
				Code:    protocol.CodeInvalidParams,
				Message: "text is required",
				Data:    protocol.InvalidConfigData{Field: "text"},
			}
		}
		return echoResult{Text: p.Text}, nil
	})
	server.Handle("boom", func(ctx context.Context, params json.RawMessage) (any, error) {
		return nil, errors.New("something broke")
	})
	server.Handle("unsupported", func(ctx context.Context, params json.RawMessage) (any, error) {
		return nil, Errorf(protocol.CodeUnsupported, "dns01 is not implemented")
	})
	server.Handle("panics", func(ctx context.Context, params json.RawMessage) (any, error) {
		panic("kaboom")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Call(ctx, "needs.params", echoParams{}, nil)
	perr, ok := AsProtocolError(err)
	require.True(t, ok)
	assert.Equal(t, protocol.CodeInvalidParams, perr.Code)
	assert.Equal(t, "text is required", perr.Message)
	assert.NotNil(t, perr.Data)

	err = client.Call(ctx, "boom", nil, nil)
	perr, ok = AsProtocolError(err)
	require.True(t, ok)
	assert.Equal(t, protocol.CodeInternalError, perr.Code)
	assert.Equal(t, "something broke", perr.Message)

	err = client.Call(ctx, "unsupported", nil, nil)
	assert.True(t, IsUnsupported(err))

	err = client.Call(ctx, "panics", nil, nil)
	perr, ok = AsProtocolError(err)
	require.True(t, ok)
	assert.Equal(t, protocol.CodeInternalError, perr.Code)
	assert.Contains(t, perr.Message, "kaboom")
}

func TestOversizeOutgoingMessageIsRejected(t *testing.T) {
	client, _ := pipePair(t, WithMaxMessageSize(4096))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Call(ctx, "echo", echoParams{Text: strings.Repeat("x", 8192)}, nil)
	assert.ErrorIs(t, err, ErrMessageTooLarge)

	err = client.Notify(ctx, "echo", echoParams{Text: strings.Repeat("x", 8192)})
	assert.ErrorIs(t, err, ErrMessageTooLarge)
}

func TestOversizeIncomingMessageClosesConn(t *testing.T) {
	serverIn, clientOut := io.Pipe()
	_, serverOut := io.Pipe()

	server := NewConn(serverIn, serverOut, WithMaxMessageSize(1024))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(ctx) }()

	go func() {
		payload := `{"jsonrpc":"2.0","id":1,"method":"echo","params":{"text":"` + strings.Repeat("x", 4096) + `"}}` + "\n"
		_, _ = clientOut.Write([]byte(payload))
	}()

	select {
	case err := <-serveErr:
		assert.ErrorIs(t, err, ErrMessageTooLarge)
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not fail on an oversize frame")
	}

	assert.ErrorIs(t, server.Call(context.Background(), "echo", nil, nil), ErrMessageTooLarge)
}

func TestCallContextCancel(t *testing.T) {
	client, server := pipePair(t)

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	server.Handle("hang", func(ctx context.Context, params json.RawMessage) (any, error) {
		<-release
		return echoResult{Text: "late"}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := client.Call(ctx, "hang", nil, nil)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// A late reply for a dropped id must not break later calls.
	server.Handle("echo", func(ctx context.Context, params json.RawMessage) (any, error) {
		return echoResult{Text: "ok"}, nil
	})
	callCtx, callCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer callCancel()
	var res echoResult
	require.NoError(t, client.Call(callCtx, "echo", nil, &res))
	assert.Equal(t, "ok", res.Text)
}

// stuckWriter never completes a write until it is closed, like a plugin that
// stopped draining its stdin.
type stuckWriter struct {
	release chan struct{}
	once    sync.Once
}

func newStuckWriter() *stuckWriter {
	return &stuckWriter{release: make(chan struct{})}
}

func (w *stuckWriter) Write(p []byte) (int, error) {
	<-w.release
	return 0, io.ErrClosedPipe
}

func (w *stuckWriter) Close() error {
	w.once.Do(func() { close(w.release) })
	return nil
}

func TestCallHonoursContextWhileTheWriteBlocks(t *testing.T) {
	writer := newStuckWriter()
	reader, _ := io.Pipe()
	conn := NewConn(reader, writer)
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := conn.Call(ctx, "hang", nil, nil)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// The writer still holds the first frame, so the next one waits for its
	// turn and gives up the same way.
	notifyCtx, notifyCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer notifyCancel()
	assert.ErrorIs(t, conn.Notify(notifyCtx, "ping", nil), context.DeadlineExceeded)

	// Closing the connection closes the writer, which releases the frame.
	require.NoError(t, conn.Close())
	select {
	case <-writer.release:
	default:
		t.Fatal("closing the connection did not close the writer")
	}
	assert.ErrorIs(t, conn.Notify(context.Background(), "ping", nil), ErrClosed)
}

func TestEOFFailsPendingCalls(t *testing.T) {
	clientIn, serverOut := io.Pipe()
	serverIn, clientOut := io.Pipe()

	client := NewConn(clientIn, clientOut)
	server := NewConn(serverIn, serverOut)
	server.Handle("hang", func(ctx context.Context, params json.RawMessage) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = client.Serve(ctx) }()
	go func() { _ = server.Serve(ctx) }()

	done := make(chan error, 1)
	go func() { done <- client.Call(context.Background(), "hang", nil, nil) }()

	// Give the call time to register before tearing the peer down.
	time.Sleep(50 * time.Millisecond)
	_ = server.Close()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, ErrClosed)
	case <-time.After(5 * time.Second):
		t.Fatal("pending call was not failed on EOF")
	}

	assert.ErrorIs(t, client.Call(context.Background(), "echo", nil, nil), ErrClosed)
}

func TestMalformedInput(t *testing.T) {
	serverIn, clientOut := io.Pipe()
	clientIn, serverOut := io.Pipe()

	server := NewConn(serverIn, serverOut)
	server.Handle("echo", func(ctx context.Context, params json.RawMessage) (any, error) {
		return echoResult{Text: "ok"}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = server.Serve(ctx) }()
	t.Cleanup(func() { _ = server.Close() })

	go func() {
		// Stray output, then a request-looking broken frame, then a good one.
		_, _ = clientOut.Write([]byte("not json at all\n"))
		_, _ = clientOut.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"echo"` + "\n"))
		_, _ = clientOut.Write([]byte(`{"jsonrpc":"2.0","id":2,"method":"echo"}` + "\n"))
	}()

	decoder := json.NewDecoder(clientIn)
	// Both broken lines are answered with a parse error carrying a null id.
	for range 2 {
		var parseResp struct {
			ID    any             `json:"id"`
			Error *protocol.Error `json:"error"`
		}
		require.NoError(t, decoder.Decode(&parseResp))
		require.NotNil(t, parseResp.Error)
		assert.Equal(t, protocol.CodeParseError, parseResp.Error.Code)
		assert.Nil(t, parseResp.ID)
	}

	var okResp struct {
		ID     float64    `json:"id"`
		Result echoResult `json:"result"`
	}
	require.NoError(t, decoder.Decode(&okResp))
	assert.Equal(t, float64(2), okResp.ID)
	assert.Equal(t, "ok", okResp.Result.Text)
}

func TestConnSatisfiesCaller(t *testing.T) {
	var _ Caller = (*Conn)(nil)
}
