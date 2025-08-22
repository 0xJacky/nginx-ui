package indexer

import (
	"context"
	"sync"
	"time"
)

// BatchWriter provides efficient batch operations for indexing
type BatchWriter struct {
	indexer   Indexer
	documents []*Document
	maxSize   int
	mutex     sync.Mutex
}

// NewBatchWriter creates a new batch writer
func NewBatchWriter(indexer Indexer, maxSize int) *BatchWriter {
	if maxSize <= 0 {
		maxSize = 1000
	}

	return &BatchWriter{
		indexer:   indexer,
		documents: make([]*Document, 0, maxSize),
		maxSize:   maxSize,
	}
}

// Add adds a document to the batch
func (bw *BatchWriter) Add(doc *Document) error {
	if doc == nil {
		return nil
	}

	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	bw.documents = append(bw.documents, doc)

	// Auto-flush if batch is full
	if len(bw.documents) >= bw.maxSize {
		return bw.flushLocked()
	}

	return nil
}

// Flush processes all documents in the batch
func (bw *BatchWriter) Flush() (*IndexResult, error) {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	if len(bw.documents) == 0 {
		return &IndexResult{}, nil
	}

	startTime := time.Now()

	// Make a copy of documents to avoid race conditions
	docs := make([]*Document, len(bw.documents))
	copy(docs, bw.documents)

	// Clear the batch
	bw.documents = bw.documents[:0]

	// Process the batch synchronously for rebuilds to ensure completion.
	err := bw.indexer.IndexDocuments(context.Background(), docs)

	result := &IndexResult{
		Processed: len(docs),
		Duration:  time.Since(startTime),
	}

	if err != nil {
		result.Failed = len(docs)
		result.ErrorRate = 1.0
		return result, err
	}

	result.Succeeded = len(docs)
	result.Throughput = float64(len(docs)) / result.Duration.Seconds()

	return result, nil
}

// flushLocked performs the flush operation while holding the mutex
func (bw *BatchWriter) flushLocked() error {
	if len(bw.documents) == 0 {
		return nil
	}

	// Make a copy of documents to avoid race conditions
	docs := make([]*Document, len(bw.documents))
	copy(docs, bw.documents)

	// Clear the batch
	bw.documents = bw.documents[:0]

	// Process the batch synchronously.
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

	bw.documents = bw.documents[:0]
}

// IsFull returns true if the batch is at maximum capacity
func (bw *BatchWriter) IsFull() bool {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	return len(bw.documents) >= bw.maxSize
}

// SetMaxSize updates the maximum batch size
func (bw *BatchWriter) SetMaxSize(size int) {
	if size <= 0 {
		return
	}

	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	bw.maxSize = size

	// If current batch exceeds new limit, resize the slice
	if len(bw.documents) > size {
		// Keep the first 'size' documents
		bw.documents = bw.documents[:size]
	}
}

// GetMaxSize returns the maximum batch size
func (bw *BatchWriter) GetMaxSize() int {
	return bw.maxSize
}
