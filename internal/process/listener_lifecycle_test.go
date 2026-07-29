package process

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type controlledListener struct {
	acceptStarted chan struct{}
	closed        chan struct{}
	acceptOnce    sync.Once
	closeOnce     sync.Once
}

func newControlledListener() *controlledListener {
	return &controlledListener{
		acceptStarted: make(chan struct{}),
		closed:        make(chan struct{}),
	}
}

func (l *controlledListener) Accept() (net.Conn, error) {
	l.acceptOnce.Do(func() {
		close(l.acceptStarted)
	})
	<-l.closed
	return nil, net.ErrClosed
}

func (l *controlledListener) Close() error {
	l.closeOnce.Do(func() {
		close(l.closed)
	})
	return nil
}

func (l *controlledListener) Addr() net.Addr {
	return controlledAddr("test")
}

type controlledAddr string

func (a controlledAddr) Network() string { return string(a) }
func (a controlledAddr) String() string  { return string(a) }

func TestLifecycleListenerSignalsWhenUnderlyingListenerCloses(t *testing.T) {
	underlying := newControlledListener()
	listener := NewLifecycleListener(underlying)
	acceptErr := make(chan error, 1)

	go func() {
		_, err := listener.Accept()
		acceptErr <- err
	}()

	select {
	case <-underlying.acceptStarted:
	case <-time.After(time.Second):
		t.Fatal("listener did not start accepting")
	}

	require.NoError(t, underlying.Close())

	select {
	case <-listener.Done():
	case <-time.After(time.Second):
		t.Fatal("listener lifecycle did not report the handover close")
	}

	select {
	case err := <-acceptErr:
		require.ErrorIs(t, err, net.ErrClosed)
	case <-time.After(time.Second):
		t.Fatal("Accept did not return after the underlying listener closed")
	}
}
