package searcher

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/blevesearch/bleve/v2"
)

const defaultSearchMemoryQuota = int64(256 * 1024 * 1024)

type searchMemoryLimiter struct {
	limit uint64
	used  atomic.Uint64
}

func newSearchMemoryLimiter(limit int64) *searchMemoryLimiter {
	if limit < 0 {
		limit = 0
	}
	return &searchMemoryLimiter{limit: uint64(limit)}
}

func (limiter *searchMemoryLimiter) acquire(size uint64) error {
	if limiter == nil || limiter.limit == 0 || size == 0 {
		return nil
	}
	if size > limiter.limit {
		return fmt.Errorf("search memory quota exceeded: estimated=%d quota=%d", size, limiter.limit)
	}
	for {
		used := limiter.used.Load()
		if used+size > limiter.limit {
			return fmt.Errorf("search memory quota exceeded: estimated=%d used=%d quota=%d", size, used, limiter.limit)
		}
		if limiter.used.CompareAndSwap(used, used+size) {
			return nil
		}
	}
}

func (limiter *searchMemoryLimiter) release(size uint64) {
	if limiter == nil || limiter.limit == 0 || size == 0 {
		return
	}
	limiter.used.Add(^uint64(size - 1))
}

func withSearchMemoryLimit(ctx context.Context, limiter *searchMemoryLimiter) context.Context {
	ctx = context.WithValue(ctx, bleve.SearchQueryStartCallbackKey, bleve.SearchQueryStartCallbackFn(limiter.acquire))
	return context.WithValue(ctx, bleve.SearchQueryEndCallbackKey, bleve.SearchQueryEndCallbackFn(func(size uint64) error {
		limiter.release(size)
		return nil
	}))
}
