package utils

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// TestEnhancedObjectPool tests the enhanced object pool functionality
func TestEnhancedObjectPool(t *testing.T) {
	pool := NewEnhancedObjectPool(
		func() *strings.Builder { return &strings.Builder{} },
		func(sb *strings.Builder) { sb.Reset() },
		10,
	)

	// Test Get/Put cycle
	sb1 := pool.Get()
	if sb1 == nil {
		t.Fatal("Pool returned nil object")
	}

	sb1.WriteString("test data")
	pool.Put(sb1)

	// Get another object (should be reused)
	sb2 := pool.Get()
	if sb2 == nil {
		t.Fatal("Pool returned nil after put")
	}

	// Should be reset
	if sb2.Len() != 0 {
		t.Error("Object not properly reset")
	}

	// Verify stats
	stats := pool.Stats()
	if stats.Reused == 0 {
		t.Error("Pool stats not tracking reuse")
	}
}

// TestStringBuilderPool tests the string builder pool
func TestStringBuilderPool(t *testing.T) {
	pool := NewStringBuilderPool(1024, 10)

	// Test basic functionality
	sb := pool.Get()
	if sb == nil {
		t.Fatal("String builder pool returned nil")
	}

	sb.WriteString("test data")
	if sb.String() != "test data" {
		t.Error("String builder not working correctly")
	}

	pool.Put(sb)

	// Test reuse
	sb2 := pool.Get()
	if sb2.Len() != 0 {
		t.Error("String builder not reset properly")
	}

	pool.Put(sb2)
}

// TestByteSlicePool tests the byte slice pool
func TestByteSlicePool(t *testing.T) {
	pool := NewByteSlicePool()

	// Test different sizes
	sizes := []int{64, 128, 256, 512, 1024}
	slices := make([][]byte, len(sizes))

	for i, size := range sizes {
		slice := pool.Get(size)
		if cap(slice) < size {
			t.Errorf("Slice capacity %d is less than requested size %d", cap(slice), size)
		}
		slices[i] = slice
	}

	// Return all slices
	for _, slice := range slices {
		pool.Put(slice)
	}

	// Test reuse
	for _, size := range sizes {
		slice := pool.Get(size)
		if cap(slice) < size {
			t.Errorf("Reused slice capacity %d is less than requested size %d", cap(slice), size)
		}
		pool.Put(slice)
	}
}

// TestMapPool tests the map pool
func TestMapPool(t *testing.T) {
	pool := NewMapPool[string, int](10, 5)

	// Test Get/Put cycle
	m1 := pool.Get()
	if m1 == nil {
		t.Fatal("Map pool returned nil")
	}

	m1["test"] = 123
	m1["another"] = 456

	if len(m1) != 2 {
		t.Error("Map not working correctly")
	}

	pool.Put(m1)

	// Test reuse and reset
	m2 := pool.Get()
	if len(m2) != 0 {
		t.Error("Map not properly reset")
	}

	pool.Put(m2)
}

// TestSlicePool tests the slice pool
func TestSlicePool(t *testing.T) {
	pool := NewSlicePool[string](10, 5)

	// Test Get/Put cycle
	slice1 := pool.Get()
	if slice1 == nil {
		t.Fatal("Slice pool returned nil")
	}

	slice1 = append(slice1, "test1", "test2")
	if len(slice1) != 2 {
		t.Error("Slice not working correctly")
	}

	pool.Put(slice1)

	// Test reuse and reset
	slice2 := pool.Get()
	if len(slice2) != 0 {
		t.Error("Slice not properly reset")
	}

	pool.Put(slice2)
}

// TestPoolManager tests the pool manager functionality
func TestPoolManager(t *testing.T) {
	manager := NewPoolManager()

	// Register a test pool
	testPool := NewStringBuilderPool(1024, 10)
	manager.RegisterPool("test_pool", testPool)

	// Retrieve the pool
	retrieved, exists := manager.GetPool("test_pool")
	if !exists {
		t.Fatal("Pool not found in manager")
	}

	if retrieved != testPool {
		t.Error("Retrieved pool is not the same as registered")
	}

	// Test non-existent pool
	_, exists = manager.GetPool("non_existent")
	if exists {
		t.Error("Non-existent pool was found")
	}

	// Test stats collection
	stats := manager.GetAllStats()
	if len(stats) == 0 {
		t.Error("No stats returned from manager")
	}
}

// TestPooledWorker tests the pooled worker functionality
func TestPooledWorker(t *testing.T) {
	worker := NewPooledWorker()
	defer worker.Cleanup()

	testData := []byte("test data for processing")
	
	processCount := 0
	processor := func(data []byte, sb *strings.Builder) error {
		sb.WriteString(string(data))
		processCount++
		return nil
	}

	// Process data multiple times
	for i := 0; i < 5; i++ {
		err := worker.ProcessWithPools(testData, processor)
		if err != nil {
			t.Fatalf("Processing failed: %v", err)
		}
	}

	if processCount != 5 {
		t.Errorf("Expected 5 processing calls, got %d", processCount)
	}
}

// BenchmarkEnhancedObjectPool benchmarks the enhanced object pool
func BenchmarkEnhancedObjectPool(b *testing.B) {
	pool := NewEnhancedObjectPool(
		func() *strings.Builder { return &strings.Builder{} },
		func(sb *strings.Builder) { sb.Reset() },
		100,
	)

	b.ResetTimer()
	b.ReportAllocs()

	b.Run("WithPooling", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sb := pool.Get()
			sb.WriteString("benchmark test data")
			_ = sb.String()
			pool.Put(sb)
		}
	})

	b.Run("WithoutPooling", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sb := &strings.Builder{}
			sb.WriteString("benchmark test data")
			_ = sb.String()
		}
	})
}

// BenchmarkStringBuilderPool benchmarks the string builder pool
func BenchmarkStringBuilderPool(b *testing.B) {
	pool := NewStringBuilderPool(1024, 100)

	b.ResetTimer()
	b.ReportAllocs()

	b.Run("PooledStringBuilder", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sb := pool.Get()
			sb.WriteString("benchmark test data for string building operations")
			sb.WriteString(" with additional content to test performance")
			_ = sb.String()
			pool.Put(sb)
		}
	})

	b.Run("RegularStringBuilder", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sb := &strings.Builder{}
			sb.Grow(1024)
			sb.WriteString("benchmark test data for string building operations")
			sb.WriteString(" with additional content to test performance")
			_ = sb.String()
		}
	})
}

// BenchmarkByteSlicePool benchmarks the byte slice pool
func BenchmarkByteSlicePool(b *testing.B) {
	pool := NewByteSlicePool()

	testSizes := []int{64, 256, 1024, 4096}

	for _, size := range testSizes {
		b.Run(f("PooledSlice_%d", size), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				slice := pool.Get(size)
				// Simulate some work
				for j := 0; j < cap(slice) && j < 100; j++ {
					slice = append(slice, byte(j))
				}
				pool.Put(slice)
			}
		})

		b.Run(f("RegularSlice_%d", size), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				slice := make([]byte, 0, size)
				// Simulate some work
				for j := 0; j < cap(slice) && j < 100; j++ {
					slice = append(slice, byte(j))
				}
			}
		})
	}
}

// BenchmarkMemoryPoolConcurrency tests concurrent access to memory pools
func BenchmarkMemoryPoolConcurrency(b *testing.B) {
	pool := NewStringBuilderPool(1024, 100)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sb := pool.Get()
			sb.WriteString("concurrent benchmark test data")
			_ = sb.String()
			pool.Put(sb)
		}
	})
}

// BenchmarkPooledWorkerPerformance benchmarks the pooled worker
func BenchmarkPooledWorkerPerformance(b *testing.B) {
	worker := NewPooledWorker()
	defer worker.Cleanup()

	testData := []byte("benchmark test data for pooled worker performance testing with longer content")

	processor := func(data []byte, sb *strings.Builder) error {
		sb.WriteString(string(data))
		sb.WriteString(" processed")
		return nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.Run("PooledWorker", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = worker.ProcessWithPools(testData, processor)
		}
	})

	b.Run("DirectProcessing", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sb := &strings.Builder{}
			sb.Grow(1024)
			sb.WriteString(string(testData))
			sb.WriteString(" processed")
		}
	})
}

// TestMemoryPoolGCPressure tests that pools reduce GC pressure
func TestMemoryPoolGCPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping GC pressure test in short mode")
	}

	pool := NewStringBuilderPool(1024, 50)

	// Force GC and get initial stats
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Perform operations with pooling
	for i := 0; i < 10000; i++ {
		sb := pool.Get()
		sb.WriteString("GC pressure test data")
		_ = sb.String()
		pool.Put(sb)
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	pooledAllocations := m2.TotalAlloc - m1.TotalAlloc

	// Reset and test without pooling
	runtime.GC()
	runtime.ReadMemStats(&m1)

	for i := 0; i < 10000; i++ {
		sb := &strings.Builder{}
		sb.Grow(1024)
		sb.WriteString("GC pressure test data")
		_ = sb.String()
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	directAllocations := m2.TotalAlloc - m1.TotalAlloc

	// Pooling should significantly reduce allocations
	if pooledAllocations >= directAllocations {
		t.Logf("Pooled allocations: %d bytes", pooledAllocations)
		t.Logf("Direct allocations: %d bytes", directAllocations)
		t.Error("Pooled allocations should be significantly less than direct allocations")
	}

	reductionRatio := float64(directAllocations-pooledAllocations) / float64(directAllocations)
	t.Logf("Memory allocation reduction: %.2f%%", reductionRatio*100)

	if reductionRatio < 0.5 { // Expect at least 50% reduction
		t.Errorf("Expected at least 50%% allocation reduction, got %.2f%%", reductionRatio*100)
	}
}

// TestGlobalPools tests the global pool instances
func TestGlobalPools(t *testing.T) {
	// Test LogStringBuilderPool
	sb := LogStringBuilderPool.Get()
	if sb == nil {
		t.Error("LogStringBuilderPool returned nil")
	}
	sb.WriteString("global pool test")
	LogStringBuilderPool.Put(sb)

	// Test GlobalByteSlicePool
	slice := GlobalByteSlicePool.Get(1024)
	if cap(slice) < 1024 {
		t.Error("GlobalByteSlicePool returned slice with insufficient capacity")
	}
	GlobalByteSlicePool.Put(slice)

	// Test StringSlicePool
	strSlice := StringSlicePool.Get()
	if strSlice == nil {
		t.Error("StringSlicePool returned nil")
	}
	strSlice = append(strSlice, "test")
	StringSlicePool.Put(strSlice)

	// Test map pools
	stringMap := StringMapPool.Get()
	if stringMap == nil {
		t.Error("StringMapPool returned nil")
	}
	stringMap["test"] = "value"
	StringMapPool.Put(stringMap)

	stringIntMap := StringIntMapPool.Get()
	if stringIntMap == nil {
		t.Error("StringIntMapPool returned nil")
	}
	stringIntMap["test"] = 123
	StringIntMapPool.Put(stringIntMap)

	intStringMap := IntStringMapPool.Get()
	if intStringMap == nil {
		t.Error("IntStringMapPool returned nil")
	}
	intStringMap[123] = "test"
	IntStringMapPool.Put(intStringMap)
}

// Helper function for formatted strings
func f(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}