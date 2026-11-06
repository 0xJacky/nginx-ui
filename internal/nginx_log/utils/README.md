# Nginx Log Performance Utils

This package provides performance optimization utilities for the nginx-ui log processing system.

## Overview

This package consolidates performance optimization code that was previously duplicated across `indexer`, `parser`, and `searcher` packages. The utilities focus on reducing memory allocations, improving concurrency, and providing efficient data structures.

## Components

### StringPool
- Provides efficient string reuse and interning to reduce memory allocations
- Thread-safe string interning with configurable limits
- Byte buffer pooling for temporary string operations

```go
pool := utils.NewStringPool()
buf := pool.Get()           // Get a reusable byte buffer
str := pool.Intern("text")  // Intern strings to reduce duplicates
pool.Put(buf)              // Return buffer to pool
```

### MemoryPool  
- Multi-size buffer pooling for different allocation needs
- Automatic size selection based on requirements
- Prevents memory fragmentation and reduces GC pressure

```go
pool := utils.NewMemoryPool()
buf := pool.Get(1024)  // Get buffer with at least 1024 bytes capacity
pool.Put(buf)          // Return buffer to appropriate pool
```

### WorkerPool
- Optimized goroutine management with bounded concurrency
- Queue-based work distribution
- Graceful shutdown support

```go
pool := utils.NewWorkerPool(10, 100) // 10 workers, 100 queue size
pool.Submit(func() { /* work */ })   // Submit work
pool.Close()                         // Shutdown gracefully
```

### BatchProcessor
- Efficient batch collection and processing
- Thread-safe operations with configurable capacity
- Automatic batch reset after retrieval

```go
bp := utils.NewBatchProcessor(100)
bp.Add(item)        // Add items to batch
batch := bp.GetBatch() // Get and reset batch
```

### MemoryOptimizer
- Memory usage monitoring and GC optimization
- Configurable thresholds and intervals
- Detailed memory statistics

```go
mo := utils.NewMemoryOptimizer(512 * 1024 * 1024) // 512MB threshold
mo.CheckMemoryUsage()  // Trigger GC if needed
stats := mo.GetMemoryStats() // Get memory statistics
```

### PerformanceMetrics
- Thread-safe performance tracking
- Operation counting, timing, and error rates
- Cache hit/miss ratio tracking

```go
pm := utils.NewPerformanceMetrics()
pm.RecordOperation(itemCount, duration, success)
pm.RecordCacheHit()
metrics := pm.GetMetrics() // Get performance snapshot
```

### Unsafe Conversions
Zero-allocation string/byte conversions for performance-critical code:
- `BytesToStringUnsafe([]byte) string`
- `StringToBytesUnsafe(string) []byte` 
- `AppendInt([]byte, int) []byte`

⚠️ **Warning**: These functions use `unsafe` operations and should be used carefully.

## Testing

The package includes comprehensive tests covering:
- Basic functionality for all components
- Concurrent access patterns
- Performance benchmarks
- Edge cases and error conditions

Run tests with:
```bash
go test ./internal/nginx_log/utils/... -v
```

Run benchmarks with:
```bash
go test ./internal/nginx_log/utils/... -bench=.
```

## Migration Notes

This package replaces the previous `performance_optimizations.go` files in:
- `internal/nginx_log/indexer/performance_optimizations.go` (removed)
- `internal/nginx_log/parser/performance_optimizations.go` (removed)
- `internal/nginx_log/searcher/performance_optimizations.go` (removed)

The consolidated implementation provides:
- Better code reuse and maintenance
- Consistent performance optimizations across packages
- Comprehensive test coverage
- Improved documentation

## Usage Guidelines

1. Use `StringPool` for frequent string operations and temporary buffers
2. Use `MemoryPool` for variable-size buffer allocations
3. Use `WorkerPool` for CPU-bound tasks requiring concurrency control
4. Use `BatchProcessor` for collecting items before bulk operations
5. Use `MemoryOptimizer` in long-running processes to manage memory
6. Use `PerformanceMetrics` to track and monitor system performance
7. Use unsafe conversions sparingly and only in performance-critical sections