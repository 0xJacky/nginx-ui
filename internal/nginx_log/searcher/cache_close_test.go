package searcher

import (
	"sync"
	"testing"
	"time"
)

// newClosableTestCache builds a cache large enough that Put is not rejected by
// the cost ceiling, so the concurrent tests below really do reach Ristretto.
func newClosableTestCache(t *testing.T) *Cache {
	t.Helper()
	cache := NewCache(1024)
	if cache == nil {
		t.Fatal("NewCache returned nil")
	}
	return cache
}

// TestCacheClearAfterCloseIsSafe is the regression test for the crash seen
// while indexing was being disabled: SwapShards clears the cache from a
// detached goroutine, so Clear can land after the searcher already closed it.
// Ristretto answers a Clear on a closed cache by sending on a closed channel,
// which panics and takes the whole process down.
func TestCacheClearAfterCloseIsSafe(t *testing.T) {
	cache := newClosableTestCache(t)
	cache.Close()

	// None of these may panic once the cache is closed.
	cache.Clear()
	cache.Put(&SearchRequest{Query: "after-close"}, &SearchResult{}, time.Minute)
	cache.PutSearchStats(&SearchRequest{Query: "after-close"}, &SearchStats{}, time.Minute)

	if result := cache.Get(&SearchRequest{Query: "after-close"}); result != nil {
		t.Fatalf("a closed cache must not return entries, got %+v", result)
	}
}

// TestCacheCloseIsIdempotent guards the second half of the same crash: the
// searcher and the service shutdown path can both close the cache.
func TestCacheCloseIsIdempotent(t *testing.T) {
	cache := newClosableTestCache(t)

	cache.Close()
	cache.Close()
}

// TestCacheOperationsRaceWithClose reproduces the original timing: writers and
// a clearer run while Close happens underneath them. Run with -race, this fails
// on the unguarded implementation with "send on closed channel".
func TestCacheOperationsRaceWithClose(t *testing.T) {
	cache := newClosableTestCache(t)

	const workers = 8
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 50; j++ {
				req := &SearchRequest{Query: "race"}
				cache.Put(req, &SearchResult{}, time.Minute)
				cache.PutSearchStats(req, &SearchStats{}, time.Minute)
				cache.Get(req)
				cache.Clear()
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		time.Sleep(time.Millisecond)
		cache.Close()
	}()

	close(start)
	wg.Wait()
}
