package helper

import (
	"sync"
	"testing"
	"time"
)

func TestDebouncer_FirstCallImmediate(t *testing.T) {
	debouncer := NewDebouncer(100 * time.Millisecond)

	var called bool
	var mu sync.Mutex

	callback := func() {
		mu.Lock()
		called = true
		mu.Unlock()
	}

	debouncer.Trigger(callback)

	// Wait a short time for the goroutine to execute
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if !called {
		t.Error("First call should execute immediately")
	}
	mu.Unlock()

	debouncer.Stop()
}

func TestDebouncer_SubsequentCallsDebounced(t *testing.T) {
	debouncer := NewDebouncer(50 * time.Millisecond)

	var callCount int
	var mu sync.Mutex

	callback := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	// First call - should execute immediately
	debouncer.Trigger(callback)
	time.Sleep(10 * time.Millisecond)

	// Multiple rapid calls - should be debounced
	debouncer.Trigger(callback)
	debouncer.Trigger(callback)
	debouncer.Trigger(callback)

	// Wait for debounce period
	time.Sleep(70 * time.Millisecond)

	mu.Lock()
	if callCount != 2 { // First immediate + one debounced
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
	mu.Unlock()

	debouncer.Stop()
}

func TestDebouncer_Stop(t *testing.T) {
	debouncer := NewDebouncer(100 * time.Millisecond)

	var called bool
	var mu sync.Mutex

	callback := func() {
		mu.Lock()
		called = true
		mu.Unlock()
	}

	// First call to set isFirst to false
	debouncer.Trigger(callback)
	time.Sleep(10 * time.Millisecond)

	// Reset called flag
	mu.Lock()
	called = false
	mu.Unlock()

	// Trigger and immediately stop
	debouncer.Trigger(callback)
	debouncer.Stop()

	// Wait longer than debounce period
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	if called {
		t.Error("Callback should not be called after Stop()")
	}
	mu.Unlock()
}
