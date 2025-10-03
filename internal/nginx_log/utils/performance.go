package utils

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// StringPool provides efficient string reuse and interning to reduce allocations and memory usage
type StringPool struct {
	pool   sync.Pool
	intern map[string]string // for string interning
	mutex  sync.RWMutex      // for intern map
}

// NewStringPool creates a new string pool
func NewStringPool() *StringPool {
	return &StringPool{
		pool: sync.Pool{
			New: func() interface{} {
				b := make([]byte, 0, 1024) // Pre-allocate 1KB
				return &b
			},
		},
		intern: make(map[string]string, 10000),
	}
}

// Get retrieves a byte buffer from the pool
func (sp *StringPool) Get() []byte {
	b := sp.pool.Get().(*[]byte)
	return (*b)[:0]
}

// Put returns a byte buffer to the pool
func (sp *StringPool) Put(b []byte) {
	if cap(b) < 32*1024 { // Don't keep very large buffers
		b = b[:0]
		sp.pool.Put(&b)
	}
}

// Intern interns a string to reduce memory duplication
func (sp *StringPool) Intern(s string) string {
	if s == "" {
		return ""
	}

	sp.mutex.RLock()
	if interned, exists := sp.intern[s]; exists {
		sp.mutex.RUnlock()
		return interned
	}
	sp.mutex.RUnlock()

	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	// Double-check after acquiring write lock
	if interned, exists := sp.intern[s]; exists {
		return interned
	}

	// Don't intern very long strings
	if len(s) > 1024 {
		return s
	}

	sp.intern[s] = s
	return s
}

// Size returns the number of interned strings
func (sp *StringPool) Size() int {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()
	return len(sp.intern)
}

// Clear clears the string pool
func (sp *StringPool) Clear() {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()
	sp.intern = make(map[string]string, 10000)
}

// MemoryPool provides memory buffer pooling to reduce GC pressure
type MemoryPool struct {
	pools []*sync.Pool
	sizes []int
}

// NewMemoryPool creates a memory pool with different buffer sizes
func NewMemoryPool() *MemoryPool {
	sizes := []int{64, 256, 1024, 4096, 16384, 65536} // Different buffer sizes
	pools := make([]*sync.Pool, len(sizes))

	for i, size := range sizes {
		s := size // Capture for closure
		pools[i] = &sync.Pool{
			New: func() interface{} {
				b := make([]byte, 0, s)
				return &b
			},
		}
	}

	return &MemoryPool{
		pools: pools,
		sizes: sizes,
	}
}

// Get retrieves a buffer of appropriate size
func (mp *MemoryPool) Get(minSize int) []byte {
	// Find the smallest pool that fits
	for i, size := range mp.sizes {
		if size >= minSize {
			buf := mp.pools[i].Get().(*[]byte)
			return (*buf)[:0] // Reset length but keep capacity
		}
	}

	// If no pool fits, allocate directly
	return make([]byte, 0, minSize)
}

// Put returns a buffer to the appropriate pool
func (mp *MemoryPool) Put(buf []byte) {
	capacity := cap(buf)

	// Find the appropriate pool
	for i, size := range mp.sizes {
		if capacity <= size {
			// Reset buffer before returning to pool
			buf = buf[:0]
			mp.pools[i].Put(&buf)
			return
		}
	}

	// Buffer too large, let GC handle it
}

// WorkerPool provides optimized worker management
type WorkerPool struct {
	workers   []Worker
	workChan  chan func()
	closeChan chan struct{}
	wg        sync.WaitGroup
}

// Worker represents a worker goroutine
type Worker struct {
	ID       int
	workChan chan func()
}

// NewWorkerPool creates an optimized worker pool
func NewWorkerPool(numWorkers int, queueSize int) *WorkerPool {
	pool := &WorkerPool{
		workers:   make([]Worker, numWorkers),
		workChan:  make(chan func(), queueSize),
		closeChan: make(chan struct{}),
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		pool.workers[i] = Worker{
			ID:       i,
			workChan: pool.workChan,
		}

		pool.wg.Add(1)
		go pool.runWorker(i)
	}

	return pool
}

// runWorker runs a single worker
func (wp *WorkerPool) runWorker(id int) {
	defer wp.wg.Done()

	for {
		select {
		case work := <-wp.workChan:
			if work != nil {
				work()
			}
		case <-wp.closeChan:
			return
		}
	}
}

// Submit submits work to the pool
func (wp *WorkerPool) Submit(work func()) bool {
	select {
	case wp.workChan <- work:
		return true
	default:
		return false // Pool is full
	}
}

// Close closes the worker pool
func (wp *WorkerPool) Close() {
	close(wp.closeChan)
	wp.wg.Wait()
}

// BatchProcessor provides efficient batch processing
type BatchProcessor struct {
	items    []interface{}
	capacity int
	mutex    sync.Mutex
}

// NewBatchProcessor creates a batch processor
func NewBatchProcessor(capacity int) *BatchProcessor {
	return &BatchProcessor{
		items:    make([]interface{}, 0, capacity),
		capacity: capacity,
	}
}

// Add adds an item to the batch
func (bp *BatchProcessor) Add(item interface{}) bool {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	if len(bp.items) >= bp.capacity {
		return false
	}

	bp.items = append(bp.items, item)
	return true
}

// GetBatch returns and clears the current batch
func (bp *BatchProcessor) GetBatch() []interface{} {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	if len(bp.items) == 0 {
		return nil
	}

	batch := make([]interface{}, len(bp.items))
	copy(batch, bp.items)
	bp.items = bp.items[:0] // Reset slice

	return batch
}

// Size returns current batch size
func (bp *BatchProcessor) Size() int {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()
	return len(bp.items)
}

// MemoryOptimizer provides memory usage optimization
type MemoryOptimizer struct {
	gcThreshold    int64 // Bytes
	lastGC         time.Time
	memStats       runtime.MemStats
	forceGCEnabled bool
}

// NewMemoryOptimizer creates a memory optimizer
func NewMemoryOptimizer(gcThreshold int64) *MemoryOptimizer {
	if gcThreshold <= 0 {
		gcThreshold = 512 * 1024 * 1024 // Default 512MB
	}
	return &MemoryOptimizer{
		gcThreshold:    gcThreshold,
		forceGCEnabled: true,
	}
}

// CheckMemoryUsage checks memory usage and triggers GC if needed
func (mo *MemoryOptimizer) CheckMemoryUsage() {
	if !mo.forceGCEnabled {
		return
	}

	runtime.ReadMemStats(&mo.memStats)

	// Check if we should force GC
	if mo.memStats.Alloc > uint64(mo.gcThreshold) && time.Since(mo.lastGC) > 30*time.Second {
		runtime.GC()
		mo.lastGC = time.Now()
	}
}

// MemoryStats represents memory statistics
type MemoryStats struct {
	AllocMB      float64 `json:"alloc_mb"`
	SysMB        float64 `json:"sys_mb"`
	HeapAllocMB  float64 `json:"heap_alloc_mb"`
	HeapSysMB    float64 `json:"heap_sys_mb"`
	GCCount      uint32  `json:"gc_count"`
	LastGCNs     uint64  `json:"last_gc_ns"`
	GCCPUPercent float64 `json:"gc_cpu_percent"`
}

// GetMemoryStats returns current memory statistics
func (mo *MemoryOptimizer) GetMemoryStats() *MemoryStats {
	runtime.ReadMemStats(&mo.memStats)

	return &MemoryStats{
		AllocMB:      float64(mo.memStats.Alloc) / 1024 / 1024,
		SysMB:        float64(mo.memStats.Sys) / 1024 / 1024,
		HeapAllocMB:  float64(mo.memStats.HeapAlloc) / 1024 / 1024,
		HeapSysMB:    float64(mo.memStats.HeapSys) / 1024 / 1024,
		GCCount:      mo.memStats.NumGC,
		LastGCNs:     mo.memStats.LastGC,
		GCCPUPercent: mo.memStats.GCCPUFraction * 100,
	}
}

// Metrics tracks general performance metrics
type Metrics struct {
	operationCount  int64
	processedItems  int64
	processTime     int64 // nanoseconds
	allocationCount int64
	allocationSize  int64
	cacheHits       int64
	cacheMisses     int64
	errorCount      int64
}

// NewMetrics creates performance metrics tracker
func NewMetrics() *Metrics {
	return &Metrics{}
}

// RecordOperation records operation metrics
func (pm *Metrics) RecordOperation(itemCount int, duration time.Duration, success bool) {
	atomic.AddInt64(&pm.operationCount, 1)
	atomic.AddInt64(&pm.processedItems, int64(itemCount))
	atomic.AddInt64(&pm.processTime, int64(duration))

	if !success {
		atomic.AddInt64(&pm.errorCount, 1)
	}
}

// RecordCacheHit records cache hit
func (pm *Metrics) RecordCacheHit() {
	atomic.AddInt64(&pm.cacheHits, 1)
}

// RecordCacheMiss records cache miss
func (pm *Metrics) RecordCacheMiss() {
	atomic.AddInt64(&pm.cacheMisses, 1)
}

// RecordAllocation records memory allocation
func (pm *Metrics) RecordAllocation(size int64) {
	atomic.AddInt64(&pm.allocationCount, 1)
	atomic.AddInt64(&pm.allocationSize, size)
}

// GetMetrics returns current metrics snapshot
func (pm *Metrics) GetMetrics() map[string]interface{} {
	operations := atomic.LoadInt64(&pm.operationCount)
	items := atomic.LoadInt64(&pm.processedItems)
	timeNs := atomic.LoadInt64(&pm.processTime)
	hits := atomic.LoadInt64(&pm.cacheHits)
	misses := atomic.LoadInt64(&pm.cacheMisses)
	errors := atomic.LoadInt64(&pm.errorCount)

	metrics := make(map[string]interface{})
	metrics["operation_count"] = operations
	metrics["processed_items"] = items
	metrics["process_time_ns"] = timeNs
	metrics["cache_hits"] = hits
	metrics["cache_misses"] = misses
	metrics["error_count"] = errors
	metrics["allocation_count"] = atomic.LoadInt64(&pm.allocationCount)
	metrics["allocation_size"] = atomic.LoadInt64(&pm.allocationSize)

	if hits+misses > 0 {
		metrics["cache_hit_rate"] = float64(hits) / float64(hits+misses)
	}

	if timeNs > 0 {
		metrics["items_per_second"] = float64(items) / (float64(timeNs) / 1e9)
		if operations > 0 {
			metrics["average_operation_time_ms"] = float64(timeNs/operations) / 1e6
		}
	}

	if operations > 0 {
		metrics["error_rate"] = float64(errors) / float64(operations)
	}

	return metrics
}

// Reset resets all metrics
func (pm *Metrics) Reset() {
	atomic.StoreInt64(&pm.operationCount, 0)
	atomic.StoreInt64(&pm.processedItems, 0)
	atomic.StoreInt64(&pm.processTime, 0)
	atomic.StoreInt64(&pm.allocationCount, 0)
	atomic.StoreInt64(&pm.allocationSize, 0)
	atomic.StoreInt64(&pm.cacheHits, 0)
	atomic.StoreInt64(&pm.cacheMisses, 0)
	atomic.StoreInt64(&pm.errorCount, 0)
}

// Unsafe conversion utilities for zero-allocation string/byte conversions
// BytesToStringUnsafe converts bytes to string without allocation
func BytesToStringUnsafe(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return *(*string)(unsafe.Pointer(&b))
}

// StringToBytesUnsafe converts string to bytes without allocation
func StringToBytesUnsafe(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return *(*[]byte)(unsafe.Pointer(&struct {
		string
		int
	}{s, len(s)}))
}

// AppendInt appends an integer to a byte slice efficiently
func AppendInt(b []byte, i int) []byte {
	// Convert int to bytes efficiently
	if i == 0 {
		return append(b, '0')
	}

	// Handle negative numbers
	if i < 0 {
		b = append(b, '-')
		i = -i
	}

	// Convert digits
	start := len(b)
	for i > 0 {
		b = append(b, byte('0'+(i%10)))
		i /= 10
	}

	// Reverse the digits
	for i, j := start, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return b
}
