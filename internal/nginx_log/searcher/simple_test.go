package searcher

import (
	"testing"
	"time"
)

// TestCache tests the basic functionality of the optimized cache
func TestCache(t *testing.T) {
	cache := NewCache(100)
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
		t.Fatal("expected cache hit")
	}

	if !cached.FromCache {
		t.Error("result should be marked as from cache")
	}

	// Test stats
	stats := cache.GetStats()
	if stats == nil {
		t.Fatal("stats should not be nil")
	}

	t.Logf("Cache stats: Size=%d, HitRate=%.2f", stats.Size, stats.HitRate)
}

func BenchmarkCacheKeyGeneration(b *testing.B) {
	cache := NewCache(1000)
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
		_ = cache.GenerateKey(req)
	}
}

// New tests to ensure cache key considers LogPaths and is order-insensitive
func TestCacheKeyIncludesLogPathsAndOrderInsensitive(t *testing.T) {
	cache := NewCache(100)
	defer cache.Close()

	st := int64(1000)
	et := int64(2000)

	reqA := &SearchRequest{
		Query:          "q",
		Limit:          10,
		Offset:         0,
		StartTime:      &st,
		EndTime:        &et,
		UseMainLogPath: true,
		LogPaths:       []string{"/var/log/nginx/a.log", "/var/log/nginx/b.log"},
		Methods:        []string{"GET"},
		StatusCodes:    []int{200, 404},
	}

	reqB := &SearchRequest{
		Query:          "q",
		Limit:          10,
		Offset:         0,
		StartTime:      &st,
		EndTime:        &et,
		UseMainLogPath: true,
		LogPaths:       []string{"/var/log/nginx/b.log", "/var/log/nginx/a.log"}, // reversed order
		Methods:        []string{"GET"},
		StatusCodes:    []int{404, 200}, // different order
	}

	keyA := cache.GenerateKey(reqA)
	keyB := cache.GenerateKey(reqB)

	if keyA != keyB {
		t.Fatalf("expected identical cache keys for order-insensitive params, got A=%s B=%s", keyA, keyB)
	}

	// Different log path should yield different key
	reqC := &SearchRequest{
		Query:          "q",
		Limit:          10,
		Offset:         0,
		StartTime:      &st,
		EndTime:        &et,
		UseMainLogPath: true,
		LogPaths:       []string{"/var/log/nginx/a.log"}, // different set
		Methods:        []string{"GET"},
		StatusCodes:    []int{200, 404},
	}

	keyC := cache.GenerateKey(reqC)
	if keyA == keyC {
		t.Fatalf("expected different cache keys when LogPaths differ, got A=%s C=%s", keyA, keyC)
	}
}

func BenchmarkCacheOperations(b *testing.B) {
	cache := NewCache(10000)
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
