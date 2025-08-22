package parser

import (
	"bufio"
	"bytes"
	"io"
	"sync"
	"unsafe"
)

// StringPool provides efficient string reuse to reduce allocations
type StringPool struct {
	pool sync.Pool
}

// NewStringPool creates a new string pool
func NewStringPool() *StringPool {
	return &StringPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 1024) // Pre-allocate 1KB
			},
		},
	}
}

// Get retrieves a byte buffer from the pool
func (sp *StringPool) Get() []byte {
	return sp.pool.Get().([]byte)[:0]
}

// Put returns a byte buffer to the pool
func (sp *StringPool) Put(b []byte) {
	if cap(b) < 32*1024 { // Don't keep very large buffers
		sp.pool.Put(b)
	}
}

// BytesToString converts bytes to string without allocation using unsafe
func BytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return *(*string)(unsafe.Pointer(&b))
}

// StringToBytes converts string to bytes without allocation using unsafe
func StringToBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return *(*[]byte)(unsafe.Pointer(&struct {
		string
		int
	}{s, len(s)}))
}

// FastScanner provides optimized line scanning with reduced allocations
type FastScanner struct {
	reader   *bufio.Reader
	buffer   []byte
	pos      int
	linePool *StringPool
}

// NewFastScanner creates an optimized scanner
func NewFastScanner(r io.Reader, bufferSize int) *FastScanner {
	return &FastScanner{
		reader:   bufio.NewReaderSize(r, bufferSize),
		buffer:   make([]byte, 0, bufferSize),
		linePool: NewStringPool(),
	}
}

// ScanLine reads the next line efficiently
func (fs *FastScanner) ScanLine() ([]byte, error) {
	line, err := fs.reader.ReadSlice('\n')
	if err != nil {
		if err == io.EOF && len(line) > 0 {
			// Return the last line without newline
			if len(line) > 0 && line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			return line, nil
		}
		return nil, err
	}
	
	// Remove newline characters
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	
	return line, nil
}

// FieldExtractor provides optimized field extraction from log lines
type FieldExtractor struct {
	fieldBuf []byte
	indices  []int
}

// NewFieldExtractor creates a field extractor
func NewFieldExtractor() *FieldExtractor {
	return &FieldExtractor{
		fieldBuf: make([]byte, 0, 512),
		indices:  make([]int, 0, 32),
	}
}

// ExtractQuotedField extracts a quoted field efficiently
func (fe *FieldExtractor) ExtractQuotedField(line []byte, start int) (field []byte, end int) {
	if start >= len(line) || line[start] != '"' {
		return nil, start
	}
	
	pos := start + 1
	fe.fieldBuf = fe.fieldBuf[:0] // Reset buffer
	
	for pos < len(line) {
		if line[pos] == '"' {
			// End of quoted field
			return fe.fieldBuf, pos + 1
		} else if line[pos] == '\\' && pos+1 < len(line) {
			// Escaped character
			fe.fieldBuf = append(fe.fieldBuf, line[pos+1])
			pos += 2
		} else {
			fe.fieldBuf = append(fe.fieldBuf, line[pos])
			pos++
		}
	}
	
	// Unclosed quote
	return fe.fieldBuf, len(line)
}

// ExtractField extracts a space-separated field
func (fe *FieldExtractor) ExtractField(line []byte, start int) (field []byte, end int) {
	// Skip leading spaces
	for start < len(line) && line[start] == ' ' {
		start++
	}
	
	if start >= len(line) {
		return nil, start
	}
	
	// Find end of field
	end = start
	for end < len(line) && line[end] != ' ' {
		end++
	}
	
	return line[start:end], end
}

// ParsedFieldCache provides LRU cache for parsed field values
type ParsedFieldCache struct {
	cache map[string]interface{}
	order []string
	mutex sync.RWMutex
	size  int
	max   int
}

// NewParsedFieldCache creates a field cache
func NewParsedFieldCache(maxSize int) *ParsedFieldCache {
	return &ParsedFieldCache{
		cache: make(map[string]interface{}, maxSize),
		order: make([]string, 0, maxSize),
		max:   maxSize,
	}
}

// Get retrieves a value from cache
func (pfc *ParsedFieldCache) Get(key string) (interface{}, bool) {
	pfc.mutex.RLock()
	defer pfc.mutex.RUnlock()
	
	val, exists := pfc.cache[key]
	return val, exists
}

// Set stores a value in cache
func (pfc *ParsedFieldCache) Set(key string, value interface{}) {
	pfc.mutex.Lock()
	defer pfc.mutex.Unlock()
	
	// Check if key already exists
	if _, exists := pfc.cache[key]; exists {
		pfc.cache[key] = value
		return
	}
	
	// Evict if at capacity
	if pfc.size >= pfc.max {
		// Remove oldest entry
		oldestKey := pfc.order[0]
		delete(pfc.cache, oldestKey)
		pfc.order = pfc.order[1:]
		pfc.size--
	}
	
	// Add new entry
	pfc.cache[key] = value
	pfc.order = append(pfc.order, key)
	pfc.size++
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
	ID      int
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

// OptimizedRegexMatcher provides compiled regex patterns with caching
type OptimizedRegexMatcher struct {
	patterns map[string]*CompiledPattern
	mutex    sync.RWMutex
}

// CompiledPattern wraps regex with metadata
type CompiledPattern struct {
	Pattern   string
	Compiled  interface{} // Could be *regexp.Regexp or optimized version
	UseCount  int64
	LastUsed  int64
}

// NewOptimizedRegexMatcher creates a regex matcher
func NewOptimizedRegexMatcher() *OptimizedRegexMatcher {
	return &OptimizedRegexMatcher{
		patterns: make(map[string]*CompiledPattern),
	}
}

// Match performs optimized pattern matching
func (orm *OptimizedRegexMatcher) Match(pattern string, text []byte) bool {
	orm.mutex.RLock()
	compiled, exists := orm.patterns[pattern]
	orm.mutex.RUnlock()
	
	if !exists {
		// Compile and cache pattern
		orm.mutex.Lock()
		// Double-check after acquiring write lock
		if compiled, exists = orm.patterns[pattern]; !exists {
			// TODO: Implement pattern compilation
			compiled = &CompiledPattern{
				Pattern:  pattern,
				UseCount: 1,
			}
			orm.patterns[pattern] = compiled
		}
		orm.mutex.Unlock()
	}
	
	// Update usage stats atomically
	compiled.UseCount++
	
	// TODO: Implement actual matching logic
	return bytes.Contains(text, StringToBytes(pattern))
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
				return make([]byte, 0, s)
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
			buf := mp.pools[i].Get().([]byte)
			return buf[:0] // Reset length but keep capacity
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
			mp.pools[i].Put(buf)
			return
		}
	}
	
	// Buffer too large, let GC handle it
}

// Performance monitoring utilities
type PerformanceMetrics struct {
	ParsedLines      int64
	ParsedBytes      int64
	ParseTime        int64 // nanoseconds
	AllocationCount  int64
	AllocationSize   int64
	CacheHits        int64
	CacheMisses      int64
	WorkerUtilization map[int]float64
	mutex            sync.RWMutex
}

// NewPerformanceMetrics creates performance metrics tracker
func NewPerformanceMetrics() *PerformanceMetrics {
	return &PerformanceMetrics{
		WorkerUtilization: make(map[int]float64),
	}
}

// RecordParse records parsing metrics
func (pm *PerformanceMetrics) RecordParse(lines int, bytes int64, duration int64) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	pm.ParsedLines += int64(lines)
	pm.ParsedBytes += bytes
	pm.ParseTime += duration
}

// RecordCacheHit records cache hit
func (pm *PerformanceMetrics) RecordCacheHit() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.CacheHits++
}

// RecordCacheMiss records cache miss
func (pm *PerformanceMetrics) RecordCacheMiss() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.CacheMisses++
}

// GetMetrics returns current metrics snapshot
func (pm *PerformanceMetrics) GetMetrics() map[string]interface{} {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	metrics := make(map[string]interface{})
	metrics["parsed_lines"] = pm.ParsedLines
	metrics["parsed_bytes"] = pm.ParsedBytes
	metrics["parse_time_ns"] = pm.ParseTime
	metrics["cache_hits"] = pm.CacheHits
	metrics["cache_misses"] = pm.CacheMisses
	
	if pm.CacheHits+pm.CacheMisses > 0 {
		metrics["cache_hit_rate"] = float64(pm.CacheHits) / float64(pm.CacheHits+pm.CacheMisses)
	}
	
	if pm.ParseTime > 0 {
		metrics["lines_per_second"] = float64(pm.ParsedLines) / (float64(pm.ParseTime) / 1e9)
		metrics["bytes_per_second"] = float64(pm.ParsedBytes) / (float64(pm.ParseTime) / 1e9)
	}
	
	return metrics
}