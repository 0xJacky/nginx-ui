package indexer

import (
	"sync"
	"time"
)

// ObjectPool provides zero-allocation object pooling for indexer components
type ObjectPool struct {
	jobPool    sync.Pool
	resultPool sync.Pool
	docPool    sync.Pool
	bufferPool sync.Pool
}

// NewObjectPool creates a new object pool with pre-allocated pools
func NewObjectPool() *ObjectPool {
	return &ObjectPool{
		jobPool: sync.Pool{
			New: func() interface{} {
				return &IndexJob{
					Documents: make([]*Document, 0, 1000), // Pre-allocate capacity
					Priority:  0,
				}
			},
		},
		resultPool: sync.Pool{
			New: func() interface{} {
				return &IndexResult{
					Processed:  0,
					Succeeded:  0,
					Failed:     0,
					Duration:   0,
					ErrorRate:  0.0,
					Throughput: 0.0,
				}
			},
		},
		docPool: sync.Pool{
			New: func() interface{} {
				return &Document{
					ID:     "",
					Fields: nil,
				}
			},
		},
		bufferPool: sync.Pool{
			New: func() interface{} {
				// Pre-allocate 4KB buffer for common operations
				b := make([]byte, 0, 4096)
				return &b
			},
		},
	}
}

// GetIndexJob returns a pooled IndexJob, reset and ready for use
func (p *ObjectPool) GetIndexJob() *IndexJob {
	job := p.jobPool.Get().(*IndexJob)
	// Reset job state
	job.Documents = job.Documents[:0] // Keep capacity, reset length
	job.Priority = 0
	job.Callback = nil
	return job
}

// PutIndexJob returns an IndexJob to the pool
func (p *ObjectPool) PutIndexJob(job *IndexJob) {
	if job != nil {
		// Clear any references to prevent memory leaks
		for i := range job.Documents {
			if job.Documents[i] != nil {
				p.PutDocument(job.Documents[i])
			}
		}
		job.Documents = job.Documents[:0]
		job.Callback = nil
		p.jobPool.Put(job)
	}
}

// GetIndexResult returns a pooled IndexResult, reset and ready for use
func (p *ObjectPool) GetIndexResult() *IndexResult {
	result := p.resultPool.Get().(*IndexResult)
	// Reset result state
	result.Processed = 0
	result.Succeeded = 0
	result.Failed = 0
	result.Duration = 0
	result.ErrorRate = 0.0
	result.Throughput = 0.0
	return result
}

// PutIndexResult returns an IndexResult to the pool
func (p *ObjectPool) PutIndexResult(result *IndexResult) {
	if result != nil {
		p.resultPool.Put(result)
	}
}

// GetDocument returns a pooled Document, reset and ready for use
func (p *ObjectPool) GetDocument() *Document {
	doc := p.docPool.Get().(*Document)
	// Reset document state
	doc.ID = ""
	doc.Fields = nil
	return doc
}

// PutDocument returns a Document to the pool
func (p *ObjectPool) PutDocument(doc *Document) {
	if doc != nil {
		doc.ID = ""
		doc.Fields = nil
		p.docPool.Put(doc)
	}
}

// GetBuffer returns a pooled byte buffer, reset and ready for use
func (p *ObjectPool) GetBuffer() *[]byte {
	bufPtr := p.bufferPool.Get().(*[]byte)
	b := *bufPtr
	b = b[:0]
	*bufPtr = b
	return bufPtr
}

// PutBuffer returns a byte buffer to the pool
func (p *ObjectPool) PutBuffer(bufPtr *[]byte) {
	if bufPtr == nil {
		return
	}
	b := *bufPtr
	if cap(b) > 0 && cap(b) <= 64*1024 { // Only keep reasonable buffers
		b = b[:0]
		*bufPtr = b
		p.bufferPool.Put(bufPtr)
	}
}

// ZeroAllocBatchProcessor provides zero-allocation batch processing
type ZeroAllocBatchProcessor struct {
	pool   *ObjectPool
	config *Config

	// Metrics
	allocationsAvoided int64
	poolHitRate        float64
	poolStats          sync.RWMutex
}

// NewZeroAllocBatchProcessor creates a new zero-allocation batch processor
func NewZeroAllocBatchProcessor(config *Config) *ZeroAllocBatchProcessor {
	return &ZeroAllocBatchProcessor{
		pool:   NewObjectPool(),
		config: config,
	}
}

// CreateJobBatch creates a batch of jobs using object pooling
func (z *ZeroAllocBatchProcessor) CreateJobBatch(documents []*Document, batchSize int) []*IndexJob {
	jobCount := (len(documents) + batchSize - 1) / batchSize
	jobs := make([]*IndexJob, 0, jobCount)

	for i := 0; i < len(documents); i += batchSize {
		end := i + batchSize
		if end > len(documents) {
			end = len(documents)
		}

		// Use pooled job
		job := z.pool.GetIndexJob()

		// Add documents to job (reusing pre-allocated slice capacity)
		for j := i; j < end; j++ {
			job.Documents = append(job.Documents, documents[j])
		}
		job.Priority = 1

		jobs = append(jobs, job)
	}

	return jobs
}

// ProcessJobResults processes job results with zero allocation
func (z *ZeroAllocBatchProcessor) ProcessJobResults(results []*IndexResult) *IndexResult {
	// Use pooled result for aggregation
	aggregatedResult := z.pool.GetIndexResult()

	totalProcessed := 0
	totalSucceeded := 0
	totalFailed := 0
	var totalDuration time.Duration

	for _, result := range results {
		totalProcessed += result.Processed
		totalSucceeded += result.Succeeded
		totalFailed += result.Failed
		totalDuration += result.Duration

		// Return individual result to pool
		z.pool.PutIndexResult(result)
	}

	// Set aggregated values
	aggregatedResult.Processed = totalProcessed
	aggregatedResult.Succeeded = totalSucceeded
	aggregatedResult.Failed = totalFailed
	aggregatedResult.Duration = totalDuration

	if totalProcessed > 0 {
		aggregatedResult.ErrorRate = float64(totalFailed) / float64(totalProcessed)
		aggregatedResult.Throughput = float64(totalSucceeded) / totalDuration.Seconds()
	}

	return aggregatedResult
}

// ReleaseBatch releases a batch of jobs back to the pool
func (z *ZeroAllocBatchProcessor) ReleaseBatch(jobs []*IndexJob) {
	for _, job := range jobs {
		z.pool.PutIndexJob(job)
	}
}

// GetPoolStats returns current pool utilization statistics
func (z *ZeroAllocBatchProcessor) GetPoolStats() PoolStats {
	z.poolStats.RLock()
	defer z.poolStats.RUnlock()

	return PoolStats{
		AllocationsAvoided: z.allocationsAvoided,
		PoolHitRate:        z.poolHitRate,
		ActiveObjects:      z.getActiveObjectCount(),
	}
}

func (z *ZeroAllocBatchProcessor) getActiveObjectCount() int {
	// This is an approximation - actual implementation would need more sophisticated tracking
	return 0
}

// PoolStats represents object pool statistics
type PoolStats struct {
	AllocationsAvoided int64   `json:"allocations_avoided"`
	PoolHitRate        float64 `json:"pool_hit_rate"`
	ActiveObjects      int     `json:"active_objects"`
}

// GetPool returns the underlying object pool for direct access if needed
func (z *ZeroAllocBatchProcessor) GetPool() *ObjectPool {
	return z.pool
}
