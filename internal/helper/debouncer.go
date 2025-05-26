package helper

import (
	"sync"
	"time"
)

// Debouncer handles debounced execution of functions
type Debouncer struct {
	timer    *time.Timer
	mutex    sync.Mutex
	duration time.Duration
	isFirst  bool
}

// NewDebouncer creates a new debouncer with the specified duration
func NewDebouncer(duration time.Duration) *Debouncer {
	return &Debouncer{
		duration: duration,
		isFirst:  true,
	}
}

// Trigger executes the callback function with debouncing logic
// For the first call, it executes immediately
// For subsequent calls, it debounces with the configured duration
func (d *Debouncer) Trigger(callback func()) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.isFirst {
		d.isFirst = false
		go callback() // Execute immediately for first call
		return
	}

	// Stop existing timer if any
	if d.timer != nil {
		d.timer.Stop()
	}

	// Set new timer for debounced execution
	d.timer = time.AfterFunc(d.duration, func() {
		go callback()
	})
}

// Stop cancels any pending debounced execution
func (d *Debouncer) Stop() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}
