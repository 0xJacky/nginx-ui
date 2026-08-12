package sitecheck

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRefreshTestService builds a Service whose sweep is a controllable stub, so
// coalescing can be observed without issuing real HTTP probes.
func newRefreshTestService(t *testing.T, sweep func()) (*Service, context.CancelFunc) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	service := &Service{
		checker:         NewSiteChecker(DefaultCheckOptions()),
		ctx:             ctx,
		cancel:          cancel,
		settingsChanged: make(chan struct{}, 1),
		refreshSweep:    sweep,
	}
	t.Cleanup(cancel)

	return service, cancel
}

// TestRefreshSitesCoalescesConcurrentCalls guards the regression behind issue
// #1792: RefreshSites is invoked from the config post-scan callback on every
// single file change and on every periodic scan, and each call used to spawn an
// unguarded goroutine running a forced probe of every site. Overlapping sweeps
// multiplied the per-sweep concurrency limit and kept the process busy.
func TestRefreshSitesCoalescesConcurrentCalls(t *testing.T) {
	var running int32
	var peak int32
	var sweeps int32

	block := make(chan struct{})
	started := make(chan struct{}, 1)

	service, _ := newRefreshTestService(t, func() {
		atomic.AddInt32(&sweeps, 1)
		current := atomic.AddInt32(&running, 1)
		for {
			observed := atomic.LoadInt32(&peak)
			if current <= observed || atomic.CompareAndSwapInt32(&peak, observed, current) {
				break
			}
		}

		select {
		case started <- struct{}{}:
		default:
		}

		<-block
		atomic.AddInt32(&running, -1)
	})

	service.RefreshSites()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("the first sweep never started")
	}

	// Hammer the entry point the way a burst of config file writes would.
	var waitGroup sync.WaitGroup
	for i := 0; i < 50; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			service.RefreshSites()
		}()
	}
	waitGroup.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&running),
		"only one sweep may be in flight at a time")

	close(block)

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&running) == 0
	}, 2*time.Second, 10*time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&peak),
		"sweeps must never overlap")
	assert.LessOrEqual(t, atomic.LoadInt32(&sweeps), int32(2),
		"50 concurrent triggers must coalesce into the running sweep plus one follow-up")
}

// TestRefreshSitesRunsFollowUpSweep verifies that a request arriving while a
// sweep is running is not dropped: coalescing must not lose changes.
func TestRefreshSitesRunsFollowUpSweep(t *testing.T) {
	var sweeps int32
	release := make(chan struct{})
	firstStarted := make(chan struct{})

	var once sync.Once
	service, _ := newRefreshTestService(t, func() {
		if atomic.AddInt32(&sweeps, 1) == 1 {
			once.Do(func() { close(firstStarted) })
			<-release
		}
	})

	service.RefreshSites()

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("the first sweep never started")
	}

	service.RefreshSites()
	close(release)

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&sweeps) == 2
	}, 2*time.Second, 10*time.Millisecond, "the queued request must run after the current sweep")

	// State must be clean so later requests still work.
	require.Eventually(t, func() bool {
		service.refreshMu.Lock()
		defer service.refreshMu.Unlock()
		return !service.refreshRunning && !service.refreshPending
	}, 2*time.Second, 10*time.Millisecond)
}

// TestRefreshSitesDoesNotLeakGoroutines checks the shape that made the original
// implementation dangerous: one goroutine per trigger, none of them bounded.
func TestRefreshSitesDoesNotLeakGoroutines(t *testing.T) {
	var sweeps int32
	service, _ := newRefreshTestService(t, func() {
		atomic.AddInt32(&sweeps, 1)
	})

	runtime.GC()
	before := runtime.NumGoroutine()

	for i := 0; i < 300; i++ {
		service.RefreshSites()
	}

	require.Eventually(t, func() bool {
		service.refreshMu.Lock()
		defer service.refreshMu.Unlock()
		return !service.refreshRunning
	}, 5*time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		return runtime.NumGoroutine() <= before+2
	}, 5*time.Second, 10*time.Millisecond,
		"300 refresh triggers must not leave goroutines behind")

	assert.Greater(t, atomic.LoadInt32(&sweeps), int32(0))
	assert.LessOrEqual(t, atomic.LoadInt32(&sweeps), int32(300))
}

// TestRefreshSitesRecoversFromPanickingSweep makes sure a failing sweep cannot
// wedge the coalescing state and silently disable all future refreshes.
func TestRefreshSitesRecoversFromPanickingSweep(t *testing.T) {
	var sweeps int32
	service, _ := newRefreshTestService(t, func() {
		if atomic.AddInt32(&sweeps, 1) == 1 {
			panic("sweep exploded")
		}
	})

	service.RefreshSites()

	require.Eventually(t, func() bool {
		service.refreshMu.Lock()
		defer service.refreshMu.Unlock()
		return !service.refreshRunning && !service.refreshPending
	}, 2*time.Second, 10*time.Millisecond, "a panicking sweep must release the coalescing state")

	service.RefreshSites()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&sweeps) == 2
	}, 2*time.Second, 10*time.Millisecond, "refreshes must still work after a failed sweep")
}
