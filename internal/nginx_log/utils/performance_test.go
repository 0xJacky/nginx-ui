package utils

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStringPool(t *testing.T) {
	pool := NewStringPool()

	// Test basic functionality
	buf := pool.Get()
	if len(buf) != 0 {
		t.Errorf("Expected empty buffer, got length %d", len(buf))
	}

	buf = append(buf, []byte("test")...)
	pool.Put(buf)

	// Test string interning
	s1 := "test_string"
	s2 := "test_string"

	interned1 := pool.Intern(s1)
	interned2 := pool.Intern(s2)

	if interned1 != interned2 {
		t.Error("Expected same interned strings")
	}

	// Test size and clear
	if pool.Size() == 0 {
		t.Error("Expected non-zero pool size")
	}

	pool.Clear()
	if pool.Size() != 0 {
		t.Error("Expected zero pool size after clear")
	}
}

func TestMemoryPool(t *testing.T) {
	pool := NewMemoryPool()

	// Test getting different sizes
	buf1 := pool.Get(100)
	if cap(buf1) < 100 {
		t.Errorf("Expected capacity >= 100, got %d", cap(buf1))
	}

	buf2 := pool.Get(1000)
	if cap(buf2) < 1000 {
		t.Errorf("Expected capacity >= 1000, got %d", cap(buf2))
	}

	// Test putting back
	pool.Put(buf1)
	pool.Put(buf2)

	// Test very large buffer (should allocate directly)
	largeBuf := pool.Get(100000)
	if cap(largeBuf) < 100000 {
		t.Errorf("Expected capacity >= 100000, got %d", cap(largeBuf))
	}
}

func TestWorkerPool(t *testing.T) {
	numWorkers := 3
	queueSize := 10
	pool := NewWorkerPool(numWorkers, queueSize)
	defer pool.Close()

	// Test job submission and execution
	var counter int64
	var mu sync.Mutex

	for i := 0; i < 5; i++ {
		success := pool.Submit(func() {
			mu.Lock()
			counter++
			mu.Unlock()
		})
		if !success {
			t.Error("Failed to submit job")
		}
	}

	// Wait for jobs to complete
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if counter != 5 {
		t.Errorf("Expected counter = 5, got %d", counter)
	}
	mu.Unlock()
}

func TestBatchProcessor(t *testing.T) {
	capacity := 3
	bp := NewBatchProcessor(capacity)

	// Test adding items
	if !bp.Add("item1") {
		t.Error("Failed to add item1")
	}
	if !bp.Add("item2") {
		t.Error("Failed to add item2")
	}
	if !bp.Add("item3") {
		t.Error("Failed to add item3")
	}

	// Should fail to add more than capacity
	if bp.Add("item4") {
		t.Error("Should have failed to add item4")
	}

	// Test size
	if bp.Size() != 3 {
		t.Errorf("Expected size 3, got %d", bp.Size())
	}

	// Test getting batch
	batch := bp.GetBatch()
	if len(batch) != 3 {
		t.Errorf("Expected batch size 3, got %d", len(batch))
	}

	// Should be empty after getting batch
	if bp.Size() != 0 {
		t.Errorf("Expected size 0 after GetBatch, got %d", bp.Size())
	}
}

func TestMemoryOptimizer(t *testing.T) {
	mo := NewMemoryOptimizer(1024 * 1024) // 1MB threshold

	// Test stats retrieval
	stats := mo.GetMemoryStats()
	if stats == nil {
		t.Fatal("Expected non-nil memory stats")
	}

	if stats.AllocMB < 0 {
		t.Error("Expected non-negative allocated memory")
	}

	// Test check memory usage (should not panic)
	mo.CheckMemoryUsage()
}

func TestMetrics(t *testing.T) {
	pm := NewMetrics()

	// Record some operations
	pm.RecordOperation(10, time.Millisecond*100, true)
	pm.RecordOperation(20, time.Millisecond*200, false) // failure
	pm.RecordCacheHit()
	pm.RecordCacheHit()
	pm.RecordCacheMiss()
	pm.RecordAllocation(1024)

	metrics := pm.GetMetrics()

	if metrics["operation_count"] != int64(2) {
		t.Errorf("Expected 2 operations, got %v", metrics["operation_count"])
	}

	if metrics["processed_items"] != int64(30) {
		t.Errorf("Expected 30 processed items, got %v", metrics["processed_items"])
	}

	if metrics["cache_hits"] != int64(2) {
		t.Errorf("Expected 2 cache hits, got %v", metrics["cache_hits"])
	}

	if metrics["cache_misses"] != int64(1) {
		t.Errorf("Expected 1 cache miss, got %v", metrics["cache_misses"])
	}

	cacheHitRate, ok := metrics["cache_hit_rate"].(float64)
	if !ok || cacheHitRate < 0.6 || cacheHitRate > 0.7 {
		t.Errorf("Expected cache hit rate around 0.67, got %v", cacheHitRate)
	}

	// Test reset
	pm.Reset()
	resetMetrics := pm.GetMetrics()
	if resetMetrics["operation_count"] != int64(0) {
		t.Errorf("Expected 0 operations after reset, got %v", resetMetrics["operation_count"])
	}
}

func TestUnsafeConversions(t *testing.T) {
	// Test bytes to string conversion
	original := []byte("test string")
	str := BytesToStringUnsafe(original)
	if str != "test string" {
		t.Errorf("Expected 'test string', got '%s'", str)
	}

	// Test string to bytes conversion
	originalStr := "test string"
	bytes := StringToBytesUnsafe(originalStr)
	if string(bytes) != originalStr {
		t.Errorf("Expected '%s', got '%s'", originalStr, string(bytes))
	}

	// Test empty cases
	emptyStr := BytesToStringUnsafe(nil)
	if emptyStr != "" {
		t.Errorf("Expected empty string, got '%s'", emptyStr)
	}

	emptyBytes := StringToBytesUnsafe("")
	if len(emptyBytes) != 0 {
		t.Errorf("Expected empty bytes, got length %d", len(emptyBytes))
	}
}

func TestAppendInt(t *testing.T) {
	testCases := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{123, "123"},
		{-456, "-456"},
		{7890, "7890"},
	}

	for _, tc := range testCases {
		buf := make([]byte, 0, 10)
		result := AppendInt(buf, tc.input)
		if string(result) != tc.expected {
			t.Errorf("AppendInt(%d) = '%s', expected '%s'", tc.input, string(result), tc.expected)
		}
	}

	// Test appending to existing buffer
	buf := []byte("prefix:")
	result := AppendInt(buf, 42)
	if string(result) != "prefix:42" {
		t.Errorf("Expected 'prefix:42', got '%s'", string(result))
	}
}

func BenchmarkStringPool(b *testing.B) {
	pool := NewStringPool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		buf = append(buf, []byte("benchmark test")...)
		pool.Put(buf)
	}
}

func BenchmarkStringIntern(b *testing.B) {
	pool := NewStringPool()
	testStrings := []string{
		"common_string_1",
		"common_string_2",
		"common_string_3",
		"common_string_1", // duplicate
		"common_string_2", // duplicate
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := testStrings[i%len(testStrings)]
		pool.Intern(s)
	}
}

func BenchmarkMemoryPool(b *testing.B) {
	pool := NewMemoryPool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get(1024)
		pool.Put(buf)
	}
}

func BenchmarkUnsafeConversions(b *testing.B) {
	testBytes := []byte("benchmark test string for conversion")
	testString := "benchmark test string for conversion"

	b.Run("BytesToStringUnsafe", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = BytesToStringUnsafe(testBytes)
		}
	})

	b.Run("StringToBytesUnsafe", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = StringToBytesUnsafe(testString)
		}
	})

	b.Run("StandardConversion", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = string(testBytes)
		}
	})
}

func TestStringPoolConcurrency(t *testing.T) {
	pool := NewStringPool()
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// Test buffer operations
				buf := pool.Get()
				buf = append(buf, byte(id), byte(j))
				pool.Put(buf)

				// Test string interning
				s := fmt.Sprintf("test_%d_%d", id, j%10) // Limited unique strings
				pool.Intern(s)
			}
		}(i)
	}

	wg.Wait()

	// Pool should have some interned strings
	if pool.Size() == 0 {
		t.Error("Expected some interned strings after concurrent operations")
	}
}
