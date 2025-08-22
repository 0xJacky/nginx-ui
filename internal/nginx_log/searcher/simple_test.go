package searcher

import (
	"testing"
	"time"
)

// TestOptimizedCache tests the basic functionality of the optimized cache
func TestOptimizedCache(t *testing.T) {
	cache := NewOptimizedSearchCache(100)
	defer cache.Close()
	
	req := &SearchRequest{
		Query:  "test",
		Limit:  10,
		Offset: 0,
	}
	
	result := &SearchResult{
		TotalHits: 5,
		Hits: []*SearchHit{
			{ID: "doc1", Score: 1.0},
			{ID: "doc2", Score: 0.9},
		},
	}
	
	// Test cache miss
	cached := cache.Get(req)
	if cached != nil {
		t.Error("expected cache miss")
	}
	
	// Test cache put
	cache.Put(req, result, 1*time.Minute)
	
	// Test cache hit
	cached = cache.Get(req)
	if cached == nil {
		t.Error("expected cache hit")
	}
	
	if !cached.FromCache {
		t.Error("result should be marked as from cache")
	}
	
	// Test stats
	stats := cache.GetStats()
	if stats == nil {
		t.Error("stats should not be nil")
	}
	
	t.Logf("Cache stats: Size=%d, HitRate=%.2f", stats.Size, stats.HitRate)
}


func BenchmarkOptimizedCacheKeyGeneration(b *testing.B) {
	cache := NewOptimizedSearchCache(1000)
	defer cache.Close()
	
	req := &SearchRequest{
		Query:       "benchmark test query",
		Limit:       100,
		Offset:      0,
		SortBy:      "score",
		SortOrder:   SortOrderDesc,
		IPAddresses: []string{"192.168.1.1", "10.0.0.1"},
		StatusCodes: []int{200, 404, 500},
		Methods:     []string{"GET", "POST"},
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = cache.GenerateOptimizedKey(req)
	}
}

func BenchmarkOptimizedCacheOperations(b *testing.B) {
	cache := NewOptimizedSearchCache(10000)
	defer cache.Close()
	
	req := &SearchRequest{
		Query: "benchmark",
		Limit: 50,
	}
	
	result := &SearchResult{
		TotalHits: 100,
		Hits: []*SearchHit{
			{ID: "doc1", Score: 1.0},
			{ID: "doc2", Score: 0.9},
		},
	}
	
	b.Run("Put", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			testReq := &SearchRequest{
				Query:  req.Query + string(rune(i%1000)),
				Limit:  req.Limit,
				Offset: i,
			}
			cache.Put(testReq, result, 1*time.Minute)
		}
	})
	
	b.Run("Get", func(b *testing.B) {
		// Pre-populate cache
		cache.Put(req, result, 1*time.Minute)
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			_ = cache.Get(req)
		}
	})
}