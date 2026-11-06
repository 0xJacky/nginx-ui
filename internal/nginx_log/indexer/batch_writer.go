package indexer

import (
	"context"
	"sync"
	"time"
)

// BatchWriter provides efficient batch operations for indexing.
//
// A batch is flushed when it reaches maxSize documents or when the heap that
// indexing it is estimated to take reaches maxBytes, whichever comes first.
// The byte bound is the one that matters on a small host: Bleve needs tens of
// kilobytes per document while it indexes a batch, so a count chosen for
// throughput alone can need more memory than the whole machine has.
type BatchWriter struct {
	indexer      Indexer
	documents    []*Document
	pendingBytes int64
	maxSize      int
	maxBytes     int64
	mutex        sync.Mutex
}

// NewBatchWriter creates a new batch writer. maxBytes <= 0 disables the
// memory bound and leaves only the document count.
func NewBatchWriter(indexer Indexer, maxSize int, maxBytes int64) *BatchWriter {
	if maxSize <= 0 {
		maxSize = 1000
	}

	return &BatchWriter{
		indexer:   indexer,
		documents: make([]*Document, 0, maxSize),
		maxSize:   maxSize,
		maxBytes:  maxBytes,
	}
}

// Add adds a document to the batch
func (bw *BatchWriter) Add(doc *Document) error {
	if doc == nil {
		return nil
	}

	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	size := estimateDocumentBytes(doc)

	// Flush before the document that would push the batch over its memory
	// budget, not after: a batch larger than the indexer's quota is refused
	// outright rather than waited for.
	if bw.maxBytes > 0 && len(bw.documents) > 0 && bw.pendingBytes+size > bw.maxBytes {
		if err := bw.flushLocked(); err != nil {
			return err
		}
	}

	bw.documents = append(bw.documents, doc)
	bw.pendingBytes += size

	if len(bw.documents) >= bw.maxSize {
		return bw.flushLocked()
	}

	return nil
}

// Flush processes all documents in the batch
func (bw *BatchWriter) Flush() (*IndexResult, error) {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	processed := len(bw.documents)
	if processed == 0 {
		return &IndexResult{}, nil
	}

	startTime := time.Now()
	err := bw.flushLocked()

	result := &IndexResult{
		Processed: processed,
		Duration:  time.Since(startTime),
	}

	if err != nil {
		result.Failed = processed
		result.ErrorRate = 1.0
		return result, err
	}

	result.Succeeded = processed
	result.Throughput = float64(processed) / result.Duration.Seconds()

	return result, nil
}

// flushLocked indexes the pending documents synchronously while holding the mutex.
func (bw *BatchWriter) flushLocked() error {
	if len(bw.documents) == 0 {
		return nil
	}

	// The indexer keeps the slice it is given, so hand it a copy and reuse
	// the buffer for the next batch.
	docs := make([]*Document, len(bw.documents))
	copy(docs, bw.documents)

	clear(bw.documents)
	bw.documents = bw.documents[:0]
	bw.pendingBytes = 0

	return bw.indexer.IndexDocuments(context.Background(), docs)
}

// Size returns the current batch size
func (bw *BatchWriter) Size() int {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	return len(bw.documents)
}

// Reset clears the batch without processing
func (bw *BatchWriter) Reset() {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	clear(bw.documents)
	bw.documents = bw.documents[:0]
	bw.pendingBytes = 0
}

// IsFull returns true if the batch is at maximum capacity
func (bw *BatchWriter) IsFull() bool {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	return len(bw.documents) >= bw.maxSize
}
