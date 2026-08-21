package certificate

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type recordingControlWriter struct {
	err      error
	messages chan int
	closed   chan struct{}
	closeOne sync.Once
}

func newRecordingControlWriter(err error) *recordingControlWriter {
	return &recordingControlWriter{
		err:      err,
		messages: make(chan int, 1),
		closed:   make(chan struct{}),
	}
}

func (w *recordingControlWriter) WriteControl(messageType int, _ []byte, _ time.Time) error {
	if w.err != nil {
		return w.err
	}
	select {
	case w.messages <- messageType:
	default:
	}
	return nil
}

func (w *recordingControlWriter) Close() error {
	w.closeOne.Do(func() { close(w.closed) })
	return nil
}

func TestIssueCertKeepaliveSendsPingBeforeCommonIdleTimeout(t *testing.T) {
	if issueCertWSPingPeriod >= time.Minute {
		t.Fatalf("ping period = %s, want less than one minute", issueCertWSPingPeriod)
	}

	writer := newRecordingControlWriter(nil)
	stop := startIssueCertKeepalive(writer, 5*time.Millisecond)
	defer stop()

	select {
	case messageType := <-writer.messages:
		if messageType != websocket.PingMessage {
			t.Fatalf("message type = %d, want ping", messageType)
		}
	case <-time.After(time.Second):
		t.Fatal("keepalive did not send a ping")
	}
}

func TestIssueCertKeepaliveClosesConnectionAfterPingFailure(t *testing.T) {
	writer := newRecordingControlWriter(errors.New("write failed"))
	stop := startIssueCertKeepalive(writer, 5*time.Millisecond)
	defer stop()

	select {
	case <-writer.closed:
	case <-time.After(time.Second):
		t.Fatal("keepalive did not close the failed connection")
	}
}
