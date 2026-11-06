package indexer

import (
	"context"
	"fmt"
	"sync"
)

const estimatedDocumentOverhead = int64(256)

// indexingAmplification converts the raw size of a document's fields into the
// heap Bleve holds while it indexes the document.
//
// Analysis is where the memory goes, not the strings themselves: every field
// gets its own token-frequency map, phrase-searchable fields keep a position
// for each token, and every numeric field expands into a run of prefix-coded
// terms, all held until the batch's segment is built. Measured with the log
// mapping on nginx combined lines, a document whose fields add up to ~890 B
// costs ~52 KiB of heap to index (about 59x); with request paths four times
// longer, ~2.3 KiB costs ~116 KiB (about 51x). Rounded up, because an estimate
// that runs low is exactly what let a 1 GiB container be pushed past its limit.
// TestIndexingMemoryProbe re-measures it against any log file.
const indexingAmplification = int64(64)

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

// estimateDocumentsBytes estimates the heap that indexing documents will hold.
func estimateDocumentsBytes(documents []*Document) int64 {
	var total int64
	for _, document := range documents {
		total += estimateDocumentBytes(document)
	}
	return total
}

// estimateDocumentBytes estimates the heap that indexing one document will hold.
func estimateDocumentBytes(document *Document) int64 {
	if document == nil {
		return 0
	}
	raw := estimatedDocumentOverhead + int64(len(document.ID))
	if fields := document.Fields; fields != nil {
		raw += int64(
			len(fields.IP) + len(fields.RegionCode) + len(fields.Province) +
				len(fields.City) + len(fields.C1) + len(fields.C2) +
				len(fields.C3) + len(fields.C4) + len(fields.Method) + len(fields.Path) +
				len(fields.PathExact) + len(fields.Protocol) + len(fields.Referer) +
				len(fields.UserAgent) + len(fields.Browser) + len(fields.BrowserVer) +
				len(fields.OS) + len(fields.OSVersion) + len(fields.DeviceType) +
				len(fields.FilePath) + len(fields.MainLogPath) + len(fields.Raw),
		)
	}
	return raw * indexingAmplification
}
