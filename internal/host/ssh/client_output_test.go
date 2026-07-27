package ssh

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
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
