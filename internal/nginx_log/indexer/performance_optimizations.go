package indexer

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// DocumentPool provides efficient document reuse
type DocumentPool struct {
	pool sync.Pool
}

// NewDocumentPool creates a document pool
func NewDocumentPool() *DocumentPool {
	return &DocumentPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &Document{
					Fields: &LogDocument{},
				}
			},
		},
	}
}

// Get retrieves a document from pool
func (dp *DocumentPool) Get() *Document {
	doc := dp.pool.Get().(*Document)
	// Reset document fields
	*doc.Fields = LogDocument{}
	doc.ID = ""
	return doc
}

// Put returns a document to pool
func (dp *DocumentPool) Put(doc *Document) {
	dp.pool.Put(doc)
}

// FastBatch provides optimized batch operations with pre-allocation
type FastBatch struct {
	documents    []*Document
	capacity     int
	size         int
	docPool      *DocumentPool
	stringPool   *StringPool
	mutex        sync.Mutex
}

// NewFastBatch creates an optimized batch
func NewFastBatch(capacity int) *FastBatch {
	return &FastBatch{
		documents:  make([]*Document, 0, capacity),
		capacity:   capacity,
		docPool:    NewDocumentPool(),
		stringPool: NewStringPool(),
	}
}

// Add adds a document to the batch
func (fb *FastBatch) Add(doc *Document) bool {
	fb.mutex.Lock()
	defer fb.mutex.Unlock()
	
	if fb.size >= fb.capacity {
		return false
	}
	
	// Clone document to avoid sharing references
	cloned := fb.docPool.Get()
	*cloned = *doc
	*cloned.Fields = *doc.Fields
	
	fb.documents = append(fb.documents, cloned)
	fb.size++
	
	return true
}

// GetDocuments returns all documents and resets the batch
func (fb *FastBatch) GetDocuments() []*Document {
	fb.mutex.Lock()
	defer fb.mutex.Unlock()
	
	if fb.size == 0 {
		return nil
	}
	
	docs := make([]*Document, fb.size)
	copy(docs, fb.documents[:fb.size])
	
	// Return documents to pool
	for i := 0; i < fb.size; i++ {
		fb.docPool.Put(fb.documents[i])
	}
	
	fb.documents = fb.documents[:0]
	fb.size = 0
	
	return docs
}

// StringPool for string interning to reduce memory usage
type StringPool struct {
	strings map[string]string
	mutex   sync.RWMutex
}

// NewStringPool creates a string pool
func NewStringPool() *StringPool {
	return &StringPool{
		strings: make(map[string]string, 10000),
	}
}

// Intern interns a string to reduce memory duplication
func (sp *StringPool) Intern(s string) string {
	if s == "" {
		return ""
	}
	
	sp.mutex.RLock()
	if interned, exists := sp.strings[s]; exists {
		sp.mutex.RUnlock()
		return interned
	}
	sp.mutex.RUnlock()
	
	sp.mutex.Lock()
	defer sp.mutex.Unlock()
	
	// Double-check after acquiring write lock
	if interned, exists := sp.strings[s]; exists {
		return interned
	}
	
	// Don't intern very long strings
	if len(s) > 1024 {
		return s
	}
	
	sp.strings[s] = s
	return s
}

// Size returns the number of interned strings
func (sp *StringPool) Size() int {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()
	return len(sp.strings)
}

// Clear clears the string pool
func (sp *StringPool) Clear() {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()
	sp.strings = make(map[string]string, 10000)
}

// OptimizedShardManager provides enhanced shard management with performance optimizations
type OptimizedShardManager struct {
	*DefaultShardManager
	shardMetrics   map[int]*ShardMetrics
	loadBalancer   *ShardLoadBalancer
	cacheManager   *ShardCacheManager
	metricsEnabled bool
}

// ShardMetrics tracks shard-specific performance metrics
type ShardMetrics struct {
	DocumentCount  int64
	IndexTime      int64 // nanoseconds
	SearchTime     int64 // nanoseconds
	ErrorCount     int64
	LastAccess     int64 // unix timestamp
	LoadFactor     float64
}

// NewOptimizedShardManager creates an optimized shard manager
func NewOptimizedShardManager(config *IndexerConfig) *OptimizedShardManager {
	base := NewDefaultShardManager(config)
	return &OptimizedShardManager{
		DefaultShardManager: base,
		shardMetrics:       make(map[int]*ShardMetrics),
		loadBalancer:       NewShardLoadBalancer(config.ShardCount),
		cacheManager:       NewShardCacheManager(1000), // Cache 1000 shard lookups
		metricsEnabled:     config.EnableMetrics,
	}
}

// GetOptimalShard returns the optimal shard based on load balancing
func (osm *OptimizedShardManager) GetOptimalShard(key string) (int, error) {
	if !osm.metricsEnabled {
		return osm.hashFunc(key, osm.config.ShardCount), nil
	}
	
	// Use load balancer to find optimal shard
	return osm.loadBalancer.GetOptimalShard(key, osm.shardMetrics), nil
}

// RecordShardOperation records shard operation for metrics
func (osm *OptimizedShardManager) RecordShardOperation(shardID int, duration time.Duration, success bool) {
	if !osm.metricsEnabled {
		return
	}
	
	osm.mu.Lock()
	defer osm.mu.Unlock()
	
	metrics, exists := osm.shardMetrics[shardID]
	if !exists {
		metrics = &ShardMetrics{}
		osm.shardMetrics[shardID] = metrics
	}
	
	if success {
		atomic.AddInt64(&metrics.DocumentCount, 1)
		atomic.AddInt64(&metrics.IndexTime, int64(duration))
	} else {
		atomic.AddInt64(&metrics.ErrorCount, 1)
	}
	
	atomic.StoreInt64(&metrics.LastAccess, time.Now().Unix())
}

// ShardLoadBalancer provides intelligent shard selection
type ShardLoadBalancer struct {
	shardWeights []float64
	totalShards  int
	mutex        sync.RWMutex
}

// NewShardLoadBalancer creates a load balancer
func NewShardLoadBalancer(shardCount int) *ShardLoadBalancer {
	weights := make([]float64, shardCount)
	for i := range weights {
		weights[i] = 1.0 // Equal weights initially
	}
	
	return &ShardLoadBalancer{
		shardWeights: weights,
		totalShards:  shardCount,
	}
}

// GetOptimalShard selects the optimal shard based on current load
func (slb *ShardLoadBalancer) GetOptimalShard(key string, metrics map[int]*ShardMetrics) int {
	slb.mutex.RLock()
	defer slb.mutex.RUnlock()
	
	// Use consistent hashing with weighted selection
	baseShardID := DefaultHashFunc(key, slb.totalShards)
	
	// Check if base shard is overloaded
	if metric, exists := metrics[baseShardID]; exists {
		loadFactor := metric.LoadFactor
		if loadFactor > 1.5 { // Overloaded
			// Find alternative shard
			minLoad := loadFactor
			alternativeShardID := baseShardID
			
			for i := 0; i < slb.totalShards; i++ {
				if altMetric, exists := metrics[i]; exists {
					if altMetric.LoadFactor < minLoad {
						minLoad = altMetric.LoadFactor
						alternativeShardID = i
					}
				}
			}
			
			return alternativeShardID
		}
	}
	
	return baseShardID
}

// UpdateShardWeights updates shard weights based on performance
func (slb *ShardLoadBalancer) UpdateShardWeights(metrics map[int]*ShardMetrics) {
	slb.mutex.Lock()
	defer slb.mutex.Unlock()
	
	for shardID, metric := range metrics {
		if shardID < len(slb.shardWeights) {
			// Weight based on inverse load factor
			if metric.LoadFactor > 0 {
				slb.shardWeights[shardID] = 1.0 / metric.LoadFactor
			} else {
				slb.shardWeights[shardID] = 1.0
			}
		}
	}
}

// ShardCacheManager provides caching for shard lookups
type ShardCacheManager struct {
	cache    map[string]int
	maxSize  int
	mutex    sync.RWMutex
	hitCount int64
	missCount int64
}

// NewShardCacheManager creates a shard cache manager
func NewShardCacheManager(maxSize int) *ShardCacheManager {
	return &ShardCacheManager{
		cache:   make(map[string]int, maxSize),
		maxSize: maxSize,
	}
}

// Get retrieves shard ID from cache
func (scm *ShardCacheManager) Get(key string) (int, bool) {
	scm.mutex.RLock()
	defer scm.mutex.RUnlock()
	
	if shardID, exists := scm.cache[key]; exists {
		atomic.AddInt64(&scm.hitCount, 1)
		return shardID, true
	}
	
	atomic.AddInt64(&scm.missCount, 1)
	return 0, false
}

// Put stores shard ID in cache
func (scm *ShardCacheManager) Put(key string, shardID int) {
	scm.mutex.Lock()
	defer scm.mutex.Unlock()
	
	if len(scm.cache) >= scm.maxSize {
		// Simple eviction: clear cache when full
		scm.cache = make(map[string]int, scm.maxSize)
	}
	
	scm.cache[key] = shardID
}

// GetStats returns cache statistics
func (scm *ShardCacheManager) GetStats() (hitCount, missCount int64, hitRate float64) {
	hits := atomic.LoadInt64(&scm.hitCount)
	misses := atomic.LoadInt64(&scm.missCount)
	total := hits + misses
	
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}
	
	return hits, misses, hitRate
}

// WorkerQueue provides optimized worker queue with priority support
type WorkerQueue struct {
	highPriorityQueue chan *IndexJob
	normalQueue       chan *IndexJob
	lowPriorityQueue  chan *IndexJob
	workers           []*OptimizedWorker
	stopChan          chan struct{}
	wg                sync.WaitGroup
	metrics           *WorkerQueueMetrics
}

// OptimizedWorker represents an optimized worker
type OptimizedWorker struct {
	ID           int
	processor    func(*IndexJob) error
	processedJobs int64
	errorCount   int64
	isActive     int32 // atomic bool
}

// WorkerQueueMetrics tracks worker queue performance
type WorkerQueueMetrics struct {
	HighPriorityCount int64
	NormalCount       int64
	LowPriorityCount  int64
	ProcessedJobs     int64
	FailedJobs        int64
	AverageWaitTime   int64 // nanoseconds
}

// NewWorkerQueue creates an optimized worker queue
func NewWorkerQueue(workerCount int, queueSize int, processor func(*IndexJob) error) *WorkerQueue {
	wq := &WorkerQueue{
		highPriorityQueue: make(chan *IndexJob, queueSize/4),
		normalQueue:       make(chan *IndexJob, queueSize/2),
		lowPriorityQueue:  make(chan *IndexJob, queueSize/4),
		workers:           make([]*OptimizedWorker, workerCount),
		stopChan:          make(chan struct{}),
		metrics:           &WorkerQueueMetrics{},
	}
	
	// Start workers
	for i := 0; i < workerCount; i++ {
		worker := &OptimizedWorker{
			ID:        i,
			processor: processor,
		}
		wq.workers[i] = worker
		
		wq.wg.Add(1)
		go wq.runWorker(worker)
	}
	
	return wq
}

// Submit submits a job to the appropriate queue based on priority
func (wq *WorkerQueue) Submit(job *IndexJob) bool {
	switch job.Priority {
	case PriorityCritical, PriorityHigh:
		select {
		case wq.highPriorityQueue <- job:
			atomic.AddInt64(&wq.metrics.HighPriorityCount, 1)
			return true
		default:
			return false
		}
	case PriorityNormal:
		select {
		case wq.normalQueue <- job:
			atomic.AddInt64(&wq.metrics.NormalCount, 1)
			return true
		default:
			return false
		}
	default: // Low priority
		select {
		case wq.lowPriorityQueue <- job:
			atomic.AddInt64(&wq.metrics.LowPriorityCount, 1)
			return true
		default:
			return false
		}
	}
}

// runWorker runs a single worker with priority-based job selection
func (wq *WorkerQueue) runWorker(worker *OptimizedWorker) {
	defer wq.wg.Done()
	
	for {
		atomic.StoreInt32(&worker.isActive, 0) // Mark as idle
		
		var job *IndexJob
		var jobReceived bool
		
		// Priority-based job selection
		select {
		case job = <-wq.highPriorityQueue:
			jobReceived = true
		case <-wq.stopChan:
			return
		default:
			select {
			case job = <-wq.highPriorityQueue:
				jobReceived = true
			case job = <-wq.normalQueue:
				jobReceived = true
			case <-wq.stopChan:
				return
			default:
				select {
				case job = <-wq.highPriorityQueue:
					jobReceived = true
				case job = <-wq.normalQueue:
					jobReceived = true
				case job = <-wq.lowPriorityQueue:
					jobReceived = true
				case <-wq.stopChan:
					return
				}
			}
		}
		
		if jobReceived {
			atomic.StoreInt32(&worker.isActive, 1) // Mark as active
			
			startTime := time.Now()
			err := worker.processor(job)
			processingTime := time.Since(startTime)
			
			if err != nil {
				atomic.AddInt64(&worker.errorCount, 1)
				atomic.AddInt64(&wq.metrics.FailedJobs, 1)
			} else {
				atomic.AddInt64(&worker.processedJobs, 1)
				atomic.AddInt64(&wq.metrics.ProcessedJobs, 1)
			}
			
			// Call callback if provided
			if job.Callback != nil {
				job.Callback(err)
			}
			
			// Update average processing time
			atomic.StoreInt64(&wq.metrics.AverageWaitTime, int64(processingTime))
		}
	}
}

// GetWorkerStats returns worker statistics
func (wq *WorkerQueue) GetWorkerStats() []*WorkerStats {
	stats := make([]*WorkerStats, len(wq.workers))
	
	for i, worker := range wq.workers {
		isActive := atomic.LoadInt32(&worker.isActive) == 1
		status := WorkerStatusIdle
		if isActive {
			status = WorkerStatusBusy
		}
		
		stats[i] = &WorkerStats{
			ID:            worker.ID,
			ProcessedJobs: atomic.LoadInt64(&worker.processedJobs),
			ErrorCount:    atomic.LoadInt64(&worker.errorCount),
			LastActive:    time.Now().Unix(),
			Status:        status,
		}
	}
	
	return stats
}

// Close closes the worker queue
func (wq *WorkerQueue) Close() {
	close(wq.stopChan)
	wq.wg.Wait()
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
	AllocMB       float64 `json:"alloc_mb"`
	SysMB         float64 `json:"sys_mb"`
	HeapAllocMB   float64 `json:"heap_alloc_mb"`
	HeapSysMB     float64 `json:"heap_sys_mb"`
	GCCount       uint32  `json:"gc_count"`
	LastGCNs      uint64  `json:"last_gc_ns"`
	GCCPUPercent  float64 `json:"gc_cpu_percent"`
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

// IndexingOptimizer provides indexing-specific optimizations
type IndexingOptimizer struct {
	bulkIndexer    *BulkIndexer
	compressionEnabled bool
	stringPool     *StringPool
	documentPool   *DocumentPool
}

// BulkIndexer provides efficient bulk indexing operations
type BulkIndexer struct {
	buffer       []*Document
	maxBatchSize int
	flushFunc    func([]*Document) error
	mutex        sync.Mutex
}

// NewBulkIndexer creates a bulk indexer
func NewBulkIndexer(maxBatchSize int, flushFunc func([]*Document) error) *BulkIndexer {
	return &BulkIndexer{
		buffer:       make([]*Document, 0, maxBatchSize),
		maxBatchSize: maxBatchSize,
		flushFunc:    flushFunc,
	}
}

// Add adds a document to the bulk buffer
func (bi *BulkIndexer) Add(doc *Document) error {
	bi.mutex.Lock()
	defer bi.mutex.Unlock()
	
	bi.buffer = append(bi.buffer, doc)
	
	if len(bi.buffer) >= bi.maxBatchSize {
		return bi.flushLocked()
	}
	
	return nil
}

// flushLocked flushes the buffer (assumes mutex is held)
func (bi *BulkIndexer) flushLocked() error {
	if len(bi.buffer) == 0 {
		return nil
	}
	
	batch := make([]*Document, len(bi.buffer))
	copy(batch, bi.buffer)
	bi.buffer = bi.buffer[:0]
	
	return bi.flushFunc(batch)
}

// Flush manually flushes any remaining documents
func (bi *BulkIndexer) Flush() error {
	bi.mutex.Lock()
	defer bi.mutex.Unlock()
	return bi.flushLocked()
}

// BytesToStringUnsafe converts bytes to string without allocation
func BytesToStringUnsafe(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// StringToBytesUnsafe converts string to bytes without allocation
func StringToBytesUnsafe(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(
		&struct {
			string
			int
		}{s, len(s)},
	))
}