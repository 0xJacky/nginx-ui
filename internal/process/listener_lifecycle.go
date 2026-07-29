package process

import (
	"errors"
	"net"
	"sync"
)

// LifecycleListener reports when its underlying listener has been closed by
// risefront during a child handover. The HTTP server otherwise stops accepting
// requests without cancelling the program context, leaving background services
// and their file locks alive in the retired process.
type LifecycleListener struct {
	net.Listener
	done chan struct{}
	once sync.Once
}

func NewLifecycleListener(listener net.Listener) *LifecycleListener {
	return &LifecycleListener{
		Listener: listener,
		done:     make(chan struct{}),
	}
}

func (l *LifecycleListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if errors.Is(err, net.ErrClosed) {
		l.signalDone()
	}
	return conn, err
}

func (l *LifecycleListener) Close() error {
	err := l.Listener.Close()
	if err == nil || errors.Is(err, net.ErrClosed) {
		l.signalDone()
	}
	return err
}

func (l *LifecycleListener) Done() <-chan struct{} {
	return l.done
}

func (l *LifecycleListener) signalDone() {
	l.once.Do(func() {
		close(l.done)
	})
}
