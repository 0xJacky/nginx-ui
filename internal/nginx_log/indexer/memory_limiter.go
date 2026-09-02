package indexer

import (
	"context"
	"fmt"
	"sync"
)

const estimatedDocumentOverhead = int64(256)

type indexMemoryLimiter struct {
	limit   int64
	mu      sync.Mutex
	used    int64
	changed chan struct{}
}

func newIndexMemoryLimiter(limit int64) *indexMemoryLimiter {
	return &indexMemoryLimiter{
		limit:   limit,
		changed: make(chan struct{}),
	}
}

func (limiter *indexMemoryLimiter) acquire(ctx context.Context, stopped <-chan struct{}, size int64) error {
	if limiter == nil || limiter.limit <= 0 || size <= 0 {
		return nil
	}
	if size > limiter.limit {
		return fmt.Errorf("index job exceeds memory quota: estimated=%d quota=%d", size, limiter.limit)
	}

	for {
		limiter.mu.Lock()
		if limiter.used+size <= limiter.limit {
			limiter.used += size
			limiter.mu.Unlock()
			return nil
		}
		changed := limiter.changed
		limiter.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-stopped:
			return fmt.Errorf("indexer stopped")
		case <-changed:
		}
	}
}

func (limiter *indexMemoryLimiter) release(size int64) {
	if limiter == nil || limiter.limit <= 0 || size <= 0 {
		return
	}

	limiter.mu.Lock()
	limiter.used -= size
	if limiter.used < 0 {
		limiter.used = 0
	}
	close(limiter.changed)
	limiter.changed = make(chan struct{})
	limiter.mu.Unlock()
}

func (limiter *indexMemoryLimiter) usage() int64 {
	if limiter == nil {
		return 0
	}
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	return limiter.used
}

func estimateDocumentsBytes(documents []*Document) int64 {
	var total int64
	for _, document := range documents {
		if document == nil {
			continue
		}
		total += estimatedDocumentOverhead + int64(len(document.ID))
		fields := document.Fields
		if fields == nil {
			continue
		}
		total += int64(
			len(fields.IP) + len(fields.RegionCode) + len(fields.Province) +
				len(fields.City) + len(fields.Method) + len(fields.Path) +
				len(fields.PathExact) + len(fields.Protocol) + len(fields.Referer) +
				len(fields.UserAgent) + len(fields.Browser) + len(fields.BrowserVer) +
				len(fields.OS) + len(fields.OSVersion) + len(fields.DeviceType) +
				len(fields.FilePath) + len(fields.MainLogPath) + len(fields.Raw),
		)
	}
	return total
}
