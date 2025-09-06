package utils

import (
	"strings"
	"sync"
	"time"
)

// EnhancedObjectPool provides advanced object pooling with automatic cleanup and monitoring
type EnhancedObjectPool[T any] struct {
	pool        sync.Pool
	created     int64
	reused      int64
	lastCleanup time.Time
	maxSize     int
	resetFunc   func(*T)
	mutex       sync.RWMutex
}

// NewEnhancedObjectPool creates a new enhanced object pool
func NewEnhancedObjectPool[T any](newFunc func() *T, resetFunc func(*T), maxSize int) *EnhancedObjectPool[T] {
	return &EnhancedObjectPool[T]{
		pool: sync.Pool{
			New: func() interface{} {
				return newFunc()
			},
		},
		maxSize:     maxSize,
		resetFunc:   resetFunc,
		lastCleanup: time.Now(),
	}
}

// Get retrieves an object from the pool
func (p *EnhancedObjectPool[T]) Get() *T {
	obj := p.pool.Get().(*T)
	
	p.mutex.Lock()
	p.reused++
	p.mutex.Unlock()
	
	return obj
}

// Put returns an object to the pool after resetting it
func (p *EnhancedObjectPool[T]) Put(obj *T) {
	if obj == nil {
		return
	}
	
	// Reset the object if reset function is provided
	if p.resetFunc != nil {
		p.resetFunc(obj)
	}
	
	p.pool.Put(obj)
}

// Stats returns pool statistics
func (p *EnhancedObjectPool[T]) Stats() PoolStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	return PoolStats{
		Created:    p.created,
		Reused:     p.reused,
		ReuseRate:  float64(p.reused) / float64(p.created+p.reused),
		LastAccess: p.lastCleanup,
	}
}

// PoolStats contains statistics about pool usage
type PoolStats struct {
	Created    int64     `json:"created"`
	Reused     int64     `json:"reused"`
	ReuseRate  float64   `json:"reuse_rate"`
	LastAccess time.Time `json:"last_access"`
}

// StringBuilderPool provides pooled string builders
type StringBuilderPool struct {
	pool *EnhancedObjectPool[strings.Builder]
}

// NewStringBuilderPool creates a new string builder pool
func NewStringBuilderPool(initialCap, maxSize int) *StringBuilderPool {
	return &StringBuilderPool{
		pool: NewEnhancedObjectPool(
			func() *strings.Builder { 
				sb := &strings.Builder{}
				sb.Grow(initialCap)
				return sb
			},
			func(sb *strings.Builder) { sb.Reset() },
			maxSize,
		),
	}
}

// Get retrieves a string builder from the pool
func (p *StringBuilderPool) Get() *strings.Builder {
	return p.pool.Get()
}

// Put returns a string builder to the pool
func (p *StringBuilderPool) Put(sb *strings.Builder) {
	p.pool.Put(sb)
}

// ByteSlicePool provides pooled byte slices
type ByteSlicePool struct {
	pools map[int]*EnhancedObjectPool[[]byte]
	mutex sync.RWMutex
}

// NewByteSlicePool creates a new byte slice pool
func NewByteSlicePool() *ByteSlicePool {
	return &ByteSlicePool{
		pools: make(map[int]*EnhancedObjectPool[[]byte]),
	}
}

// Get retrieves a byte slice of the requested size
func (p *ByteSlicePool) Get(size int) []byte {
	// Round up to nearest power of 2 for better pooling
	poolSize := nextPowerOf2(size)
	
	p.mutex.RLock()
	pool, exists := p.pools[poolSize]
	p.mutex.RUnlock()
	
	if !exists {
		p.mutex.Lock()
		// Double-check after acquiring write lock
		if pool, exists = p.pools[poolSize]; !exists {
			pool = NewEnhancedObjectPool(
				func() *[]byte {
					slice := make([]byte, 0, poolSize)
					return &slice
				},
				func(slice *[]byte) { *slice = (*slice)[:0] },
				100, // max 100 slices per size
			)
			p.pools[poolSize] = pool
		}
		p.mutex.Unlock()
	}
	
	slice := pool.Get()
	return *slice
}

// Put returns a byte slice to the pool
func (p *ByteSlicePool) Put(slice []byte) {
	if slice == nil {
		return
	}
	
	capacity := cap(slice)
	
	p.mutex.RLock()
	pool, exists := p.pools[capacity]
	p.mutex.RUnlock()
	
	if exists {
		pool.Put(&slice)
	}
}

// nextPowerOf2 returns the next power of 2 greater than or equal to n
func nextPowerOf2(n int) int {
	if n <= 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++
	return n
}

// MapPool provides pooled maps
type MapPool[K comparable, V any] struct {
	pool *EnhancedObjectPool[map[K]V]
}

// NewMapPool creates a new map pool
func NewMapPool[K comparable, V any](initialSize, maxSize int) *MapPool[K, V] {
	return &MapPool[K, V]{
		pool: NewEnhancedObjectPool(
			func() *map[K]V {
				m := make(map[K]V, initialSize)
				return &m
			},
			func(m *map[K]V) {
				// Clear the map
				for k := range *m {
					delete(*m, k)
				}
			},
			maxSize,
		),
	}
}

// Get retrieves a map from the pool
func (p *MapPool[K, V]) Get() map[K]V {
	return *p.pool.Get()
}

// Put returns a map to the pool
func (p *MapPool[K, V]) Put(m map[K]V) {
	p.pool.Put(&m)
}

// SlicePool provides pooled slices
type SlicePool[T any] struct {
	pool *EnhancedObjectPool[[]T]
}

// NewSlicePool creates a new slice pool
func NewSlicePool[T any](initialCap, maxSize int) *SlicePool[T] {
	return &SlicePool[T]{
		pool: NewEnhancedObjectPool(
			func() *[]T {
				slice := make([]T, 0, initialCap)
				return &slice
			},
			func(slice *[]T) { *slice = (*slice)[:0] },
			maxSize,
		),
	}
}

// Get retrieves a slice from the pool
func (p *SlicePool[T]) Get() []T {
	return *p.pool.Get()
}

// Put returns a slice to the pool
func (p *SlicePool[T]) Put(slice []T) {
	p.pool.Put(&slice)
}

// PoolManager manages multiple object pools
type PoolManager struct {
	pools map[string]interface{}
	mutex sync.RWMutex
}

// NewPoolManager creates a new pool manager
func NewPoolManager() *PoolManager {
	return &PoolManager{
		pools: make(map[string]interface{}),
	}
}

// RegisterPool registers a pool with the manager
func (pm *PoolManager) RegisterPool(name string, pool interface{}) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.pools[name] = pool
}

// GetPool retrieves a pool by name
func (pm *PoolManager) GetPool(name string) (interface{}, bool) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	pool, exists := pm.pools[name]
	return pool, exists
}

// GetAllStats returns statistics for all registered pools
func (pm *PoolManager) GetAllStats() map[string]interface{} {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	stats := make(map[string]interface{})
	for name, pool := range pm.pools {
		// Try to get stats if the pool supports it
		if statsProvider, ok := pool.(interface{ Stats() PoolStats }); ok {
			stats[name] = statsProvider.Stats()
		} else {
			stats[name] = "stats not available"
		}
	}
	
	return stats
}

// Global pool manager instance
var globalPoolManager = NewPoolManager()

// GetGlobalPoolManager returns the global pool manager
func GetGlobalPoolManager() *PoolManager {
	return globalPoolManager
}

// Common pool instances for frequent use
var (
	// String builder pool for log processing
	LogStringBuilderPool = NewStringBuilderPool(1024, 50)
	
	// Byte slice pool for I/O operations
	GlobalByteSlicePool = NewByteSlicePool()
	
	// String slice pool for batch processing
	StringSlicePool = NewSlicePool[string](100, 20)
	
	// Map pools for common use cases
	StringMapPool     = NewMapPool[string, string](10, 20)
	StringIntMapPool  = NewMapPool[string, int](10, 20)
	IntStringMapPool  = NewMapPool[int, string](10, 20)
)

// Initialize global pools
func init() {
	// Register common pools with the global manager
	globalPoolManager.RegisterPool("log_string_builder", LogStringBuilderPool)
	globalPoolManager.RegisterPool("global_byte_slice", GlobalByteSlicePool)
	globalPoolManager.RegisterPool("string_slice", StringSlicePool)
	globalPoolManager.RegisterPool("string_map", StringMapPool)
	globalPoolManager.RegisterPool("string_int_map", StringIntMapPool)
	globalPoolManager.RegisterPool("int_string_map", IntStringMapPool)
}

// PooledWorker represents a worker that uses object pools
type PooledWorker struct {
	stringBuilders *StringBuilderPool
	byteSlices     *ByteSlicePool
	workBuffer     []byte
}

// NewPooledWorker creates a new pooled worker
func NewPooledWorker() *PooledWorker {
	return &PooledWorker{
		stringBuilders: LogStringBuilderPool,
		byteSlices:     GlobalByteSlicePool,
	}
}

// ProcessWithPools processes data using object pools to minimize allocations
func (pw *PooledWorker) ProcessWithPools(data []byte, processor func([]byte, *strings.Builder) error) error {
	// Get pooled string builder
	sb := pw.stringBuilders.Get()
	defer pw.stringBuilders.Put(sb)
	
	// Get pooled byte slice if needed
	if len(pw.workBuffer) < len(data) {
		if pw.workBuffer != nil {
			pw.byteSlices.Put(pw.workBuffer)
		}
		pw.workBuffer = pw.byteSlices.Get(len(data))
	}
	
	// Use pooled objects for processing
	copy(pw.workBuffer, data)
	return processor(pw.workBuffer, sb)
}

// Cleanup releases resources held by the worker
func (pw *PooledWorker) Cleanup() {
	if pw.workBuffer != nil {
		pw.byteSlices.Put(pw.workBuffer)
		pw.workBuffer = nil
	}
}