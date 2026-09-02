package ssh

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

func TestSynchronizedBufferConcurrentWrites(t *testing.T) {
	const writers = 64
	const payload = "stdout-or-stderr\n"
	var buffer synchronizedBuffer
	var wait sync.WaitGroup
	wait.Add(writers)
	for range writers {
		go func() {
			defer wait.Done()
			if _, err := buffer.Write([]byte(payload)); err != nil {
				t.Errorf("Write() error = %v", err)
			}
		}()
	}
	wait.Wait()
	if got, want := len(buffer.String()), writers*len(payload); got != want {
		t.Fatalf("combined output length = %d, want %d", got, want)
	}
}

// A client discarded after a settings change must not redial the host it was
// built for; the options were captured at construction time.
func TestClosedClientRefusesToRedial(t *testing.T) {
	client := NewClient(ClientOptions{
		Address:    "127.0.0.1:1",
		User:       "nobody",
		AuthMethod: "key",
	})
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, err := client.Exec(context.Background(), "/bin/true")
	if err == nil {
		t.Fatal("Exec on a closed client dialed the host again")
	}
	if !errors.Is(err, ErrClientClosed) && !strings.Contains(err.Error(), "closed") {
		t.Fatalf("Exec error = %v, want the closed-client error", err)
	}
}

// connectLocked must not pay a probe round trip while the keepalive goroutine
// is already exercising the connection; it only probes once the last success
// is older than the keepalive interval.
func TestNeedsProbeOnlyWhenLastSuccessIsStale(t *testing.T) {
	client := NewClient(ClientOptions{KeepAlive: 30 * time.Second})
	now := time.Now()

	if !client.needsProbe(now) {
		t.Fatal("a connection that never answered must be probed")
	}

	client.markProbed()
	if client.needsProbe(now) {
		t.Fatal("a connection that just answered must not be probed again")
	}
	if client.needsProbe(now.Add(29 * time.Second)) {
		t.Fatal("a connection inside the keepalive interval must not be probed")
	}
	if !client.needsProbe(now.Add(31 * time.Second)) {
		t.Fatal("a connection past the keepalive interval must be probed")
	}
}

// A redial for a connection that another caller already replaced must keep
// the newer, recently verified connection instead of tearing it down and
// dialing again.
func TestRedialLockedKeepsAlreadyReplacedConnection(t *testing.T) {
	client := NewClient(ClientOptions{Address: "127.0.0.1:1", KeepAlive: 30 * time.Second})
	stale := &gossh.Client{}
	newer := &gossh.Client{}
	client.conn = newer
	client.markProbed()

	conn, err := client.redialLocked(context.Background(), stale)
	if err != nil {
		t.Fatalf("redialLocked: %v", err)
	}
	if conn != newer || client.conn != newer {
		t.Fatal("redialLocked replaced a connection that was not the stale one")
	}
}
