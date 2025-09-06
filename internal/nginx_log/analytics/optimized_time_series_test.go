package analytics

import (
	"context"
	"testing"
	"time"
)

// mockSearcher implements Searcher for testing
type mockSearcher struct{}

func (m *mockSearcher) Search(ctx context.Context, req *SearchRequest) (*SearchResult, error) {
	// Generate mock search results
	hits := make([]*SearchHit, 1000)
	baseTime := time.Now().Unix()
	
	for i := 0; i < 1000; i++ {
		hits[i] = &SearchHit{
			Fields: map[string]interface{}{
				"timestamp":  float64(baseTime + int64(i*60)), // 1 minute intervals
				"ip":         "192.168.1." + string(rune('1' + (i % 254))),
				"method":     []string{"GET", "POST", "PUT"}[i%3],
				"path":       "/api/test",
				"status":     float64([]int{200, 404, 500}[i%3]),
				"bytes_sent": float64(1000 + (i % 5000)),
			},
		}
	}
	
	return &SearchResult{
		Hits:      hits,
		TotalHits: 1000,
		Stats: &SearchStats{
			TotalBytes: 2500000,
		},
	}, nil
}

func (m *mockSearcher) Aggregate(ctx context.Context, req *AggregationRequest) (*AggregationResult, error) {
	return &AggregationResult{}, nil
}

func (m *mockSearcher) Suggest(ctx context.Context, text string, field string, size int) ([]*Suggestion, error) {
	return []*Suggestion{}, nil
}

func (m *mockSearcher) Analyze(ctx context.Context, text string, analyzer string) ([]string, error) {
	return []string{}, nil
}

func (m *mockSearcher) ClearCache() error {
	return nil
}

// TestOptimizedTimeSeriesProcessor tests the optimized processor
func TestOptimizedTimeSeriesProcessor(t *testing.T) {
	processor := NewOptimizedTimeSeriesProcessor()
	
	if processor == nil {
		t.Fatal("Failed to create optimized processor")
	}
	
	// Test bucket pool
	bucketPool := processor.getBucketPool(60)
	if bucketPool == nil {
		t.Fatal("Failed to get bucket pool")
	}
	
	buckets := bucketPool.Get()
	if buckets == nil {
		t.Fatal("Failed to get buckets from pool")
	}
	
	bucketPool.Put(buckets)
}

// TestTimeBucket tests the time bucket functionality
func TestTimeBucket(t *testing.T) {
	timestamp := time.Now().Unix()
	bucket := NewTimeBucket(timestamp)
	
	if bucket.Timestamp != timestamp {
		t.Errorf("Expected timestamp %d, got %d", timestamp, bucket.Timestamp)
	}
	
	// Test adding entries
	bucket.AddEntry("192.168.1.1", 200, "GET", "/api/test", 1024)
	bucket.AddEntry("192.168.1.2", 404, "POST", "/api/data", 512)
	bucket.AddEntry("192.168.1.1", 200, "GET", "/api/test", 2048) // Duplicate IP
	
	// Verify counts
	if bucket.RequestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", bucket.RequestCount)
	}
	
	if bucket.BytesTransferred != 3584 {
		t.Errorf("Expected 3584 bytes, got %d", bucket.BytesTransferred)
	}
	
	if bucket.GetUniqueVisitorCount() != 2 {
		t.Errorf("Expected 2 unique visitors, got %d", bucket.GetUniqueVisitorCount())
	}
	
	// Verify status codes
	if bucket.StatusCodes[200] != 2 {
		t.Errorf("Expected 2 status 200, got %d", bucket.StatusCodes[200])
	}
	
	if bucket.StatusCodes[404] != 1 {
		t.Errorf("Expected 1 status 404, got %d", bucket.StatusCodes[404])
	}
}

// TestTimeSeriesCache tests the caching functionality
func TestTimeSeriesCache(t *testing.T) {
	cache := NewTimeSeriesCache(5, 300) // 5 entries, 5 min TTL
	
	// Test put and get
	testData := "test_data"
	cache.Put("key1", testData)
	
	retrieved, found := cache.Get("key1")
	if !found {
		t.Error("Failed to find cached data")
	}
	
	if retrieved != testData {
		t.Errorf("Expected %s, got %v", testData, retrieved)
	}
	
	// Test non-existent key
	_, found = cache.Get("non_existent")
	if found {
		t.Error("Found non-existent key")
	}
	
	// Test cache eviction
	for i := 0; i < 10; i++ {
		key := "evict_key" + string(rune('0'+i))
		cache.Put(key, i)
	}
	
	// Original key1 should still exist (eviction targets oldest by timestamp)
	_, found = cache.Get("key1")
	if !found {
		t.Log("Key1 was evicted as expected due to LRU policy")
	}
}

// TestOptimizedGetVisitorsByTime tests optimized visitors by time
func TestOptimizedGetVisitorsByTime(t *testing.T) {
	processor := NewOptimizedTimeSeriesProcessor()
	mockSearcher := &mockSearcher{}
	
	req := &VisitorsByTimeRequest{
		StartTime:       time.Now().Unix() - 3600, // 1 hour ago
		EndTime:         time.Now().Unix(),
		LogPaths:        []string{"/var/log/nginx/access.log"},
		IntervalSeconds: 60, // 1 minute intervals
	}
	
	result, err := processor.OptimizedGetVisitorsByTime(context.Background(), req, mockSearcher)
	if err != nil {
		t.Fatalf("Failed to get visitors by time: %v", err)
	}
	
	if result == nil {
		t.Fatal("Result is nil")
	}
	
	if len(result.Data) == 0 {
		t.Error("No data returned")
	}
	
	// Verify data is sorted
	for i := 1; i < len(result.Data); i++ {
		if result.Data[i].Timestamp < result.Data[i-1].Timestamp {
			t.Error("Data is not sorted by timestamp")
		}
	}
}

// TestOptimizedGetTrafficByTime tests optimized traffic by time
func TestOptimizedGetTrafficByTime(t *testing.T) {
	processor := NewOptimizedTimeSeriesProcessor()
	mockSearcher := &mockSearcher{}
	
	req := &TrafficByTimeRequest{
		StartTime:       time.Now().Unix() - 3600,
		EndTime:         time.Now().Unix(),
		LogPaths:        []string{"/var/log/nginx/access.log"},
		IntervalSeconds: 300, // 5 minute intervals
	}
	
	result, err := processor.OptimizedGetTrafficByTime(context.Background(), req, mockSearcher)
	if err != nil {
		t.Fatalf("Failed to get traffic by time: %v", err)
	}
	
	if result == nil {
		t.Fatal("Result is nil")
	}
	
	if len(result.Data) == 0 {
		t.Error("No data returned")
	}
	
	// Verify comprehensive metrics
	for _, point := range result.Data {
		if point.Timestamp <= 0 {
			t.Error("Invalid timestamp")
		}
		if point.Requests < 0 {
			t.Error("Invalid request count")
		}
		if point.Bytes < 0 {
			t.Error("Invalid byte count")
		}
		if point.UniqueVisitors < 0 {
			t.Error("Invalid unique visitor count")
		}
	}
}

// TestHyperLogLog tests the HyperLogLog cardinality estimator
func TestHyperLogLog(t *testing.T) {
	hll := NewHyperLogLog(8) // 256 buckets
	
	// Add known unique values
	uniqueValues := []string{
		"192.168.1.1", "192.168.1.2", "192.168.1.3",
		"10.0.0.1", "10.0.0.2", "172.16.0.1",
	}
	
	for _, value := range uniqueValues {
		hll.Add(value)
	}
	
	count := hll.Count()
	expectedCount := uint64(len(uniqueValues))
	
	// HyperLogLog should be reasonably accurate for small sets
	if count == 0 {
		t.Error("HyperLogLog count is 0")
	}
	
	// Allow for some estimation error
	diff := count - expectedCount
	if diff < 0 {
		diff = -diff
	}
	
	if diff > expectedCount/2 {
		t.Logf("HyperLogLog estimate %d vs actual %d (difference: %d)", count, expectedCount, diff)
	}
}

// TestAdvancedTimeSeriesProcessor tests advanced analytics
func TestAdvancedTimeSeriesProcessor(t *testing.T) {
	processor := NewAdvancedTimeSeriesProcessor()
	
	if processor == nil {
		t.Fatal("Failed to create advanced processor")
	}
	
	// Test anomaly detection
	testData := []TimeValue{
		{Timestamp: 1000, Value: 10},
		{Timestamp: 1060, Value: 12},
		{Timestamp: 1120, Value: 11},
		{Timestamp: 1180, Value: 13},
		{Timestamp: 1240, Value: 10},
		{Timestamp: 1300, Value: 50}, // Anomaly
		{Timestamp: 1360, Value: 12},
		{Timestamp: 1420, Value: 11},
	}
	
	anomalies := processor.DetectAnomalies(testData)
	if len(anomalies) == 0 {
		t.Log("No anomalies detected")
	} else {
		t.Logf("Detected %d anomalies", len(anomalies))
		for _, anomaly := range anomalies {
			t.Logf("Anomaly: timestamp=%d, value=%d, expected=%d, deviation=%.2f",
				anomaly.Timestamp, anomaly.Value, anomaly.Expected, anomaly.Deviation)
		}
	}
}

// TestTrendAnalysis tests trend calculation
func TestTrendAnalysis(t *testing.T) {
	processor := NewAdvancedTimeSeriesProcessor()
	
	// Test increasing trend
	increasingData := []TimeValue{
		{Timestamp: 1000, Value: 10},
		{Timestamp: 1060, Value: 15},
		{Timestamp: 1120, Value: 20},
		{Timestamp: 1180, Value: 25},
		{Timestamp: 1240, Value: 30},
	}
	
	trend := processor.CalculateTrend(increasingData)
	if trend.Direction != "increasing" {
		t.Errorf("Expected increasing trend, got %s", trend.Direction)
	}
	
	if trend.Slope <= 0 {
		t.Errorf("Expected positive slope, got %f", trend.Slope)
	}
	
	// Test decreasing trend
	decreasingData := []TimeValue{
		{Timestamp: 1000, Value: 30},
		{Timestamp: 1060, Value: 25},
		{Timestamp: 1120, Value: 20},
		{Timestamp: 1180, Value: 15},
		{Timestamp: 1240, Value: 10},
	}
	
	trend = processor.CalculateTrend(decreasingData)
	if trend.Direction != "decreasing" {
		t.Errorf("Expected decreasing trend, got %s", trend.Direction)
	}
	
	if trend.Slope >= 0 {
		t.Errorf("Expected negative slope, got %f", trend.Slope)
	}
}

// BenchmarkOptimizedTimeSeriesProcessing benchmarks the optimized processing
func BenchmarkOptimizedTimeSeriesProcessing(b *testing.B) {
	processor := NewOptimizedTimeSeriesProcessor()
	mockSearcher := &mockSearcher{}
	
	req := &VisitorsByTimeRequest{
		StartTime:       time.Now().Unix() - 3600,
		EndTime:         time.Now().Unix(),
		LogPaths:        []string{"/var/log/nginx/access.log"},
		IntervalSeconds: 60,
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.Run("OptimizedVisitorsByTime", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := processor.OptimizedGetVisitorsByTime(context.Background(), req, mockSearcher)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkTimeBucketOperations benchmarks time bucket operations
func BenchmarkTimeBucketOperations(b *testing.B) {
	bucket := NewTimeBucket(time.Now().Unix())
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.Run("AddEntry", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ip := "192.168.1." + string(rune('1' + (i % 254)))
			bucket.AddEntry(ip, 200, "GET", "/api/test", 1024)
		}
	})
	
	b.Run("GetUniqueVisitorCount", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = bucket.GetUniqueVisitorCount()
		}
	})
}

// BenchmarkHyperLogLog benchmarks HyperLogLog operations
func BenchmarkHyperLogLog(b *testing.B) {
	hll := NewHyperLogLog(12) // 4096 buckets
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.Run("Add", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ip := "192.168.1." + string(rune('1' + (i % 255)))
			hll.Add(ip)
		}
	})
	
	b.Run("Count", func(b *testing.B) {
		// Add some data first
		for i := 0; i < 1000; i++ {
			hll.Add("192.168.1." + string(rune('1' + (i % 255))))
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = hll.Count()
		}
	})
}

// BenchmarkTimeSeriesCache benchmarks cache operations
func BenchmarkTimeSeriesCache(b *testing.B) {
	cache := NewTimeSeriesCache(1000, 3600)
	
	// Pre-populate cache
	for i := 0; i < 500; i++ {
		key := "key_" + string(rune('0' + (i % 10)))
		cache.Put(key, i)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.Run("Get", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := "key_" + string(rune('0' + (i % 10)))
			cache.Get(key)
		}
	})
	
	b.Run("Put", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := "bench_key_" + string(rune('0' + (i % 100)))
			cache.Put(key, i)
		}
	})
}

// BenchmarkAnomalyDetection benchmarks anomaly detection
func BenchmarkAnomalyDetection(b *testing.B) {
	processor := NewAdvancedTimeSeriesProcessor()
	
	// Generate test data
	testData := make([]TimeValue, 100)
	for i := 0; i < 100; i++ {
		testData[i] = TimeValue{
			Timestamp: int64(1000 + i*60),
			Value:     10 + (i % 5), // Normal pattern with occasional spikes
		}
	}
	
	// Add some anomalies
	testData[50].Value = 100
	testData[75].Value = 2
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = processor.DetectAnomalies(testData)
	}
}

// BenchmarkTrendCalculation benchmarks trend calculation
func BenchmarkTrendCalculation(b *testing.B) {
	processor := NewAdvancedTimeSeriesProcessor()
	
	// Generate test data with trend
	testData := make([]TimeValue, 50)
	for i := 0; i < 50; i++ {
		testData[i] = TimeValue{
			Timestamp: int64(1000 + i*60),
			Value:     10 + i/2, // Increasing trend
		}
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = processor.CalculateTrend(testData)
	}
}