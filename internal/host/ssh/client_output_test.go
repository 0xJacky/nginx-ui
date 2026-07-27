package ssh

import (
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
