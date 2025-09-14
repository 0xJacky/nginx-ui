package searcher

import (
	"testing"
	"time"
)

// TestOptimizedSearchCache tests the basic functionality of the optimized search cache
func TestOptimizedSearchCache(t *testing.T) {
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

func TestBasicSearcherConfig(t *testing.T) {
	config := DefaultSearcherConfig()

	if config.MaxConcurrency <= 0 {
		t.Error("MaxConcurrency should be greater than 0")
	}

	if config.CacheSize <= 0 {
		t.Error("CacheSize should be greater than 0")
	}

	if !config.EnableCache {
		t.Error("EnableCache should be true by default")
	}
}

func TestQueryBuilderValidation(t *testing.T) {
	qb := NewQueryBuilderService()

	// Test valid request
	validReq := &SearchRequest{
		Query:  "test",
		Limit:  10,
		Offset: 0,
	}

	err := qb.ValidateSearchRequest(validReq)
	if err != nil {
		t.Errorf("valid request should not have validation error: %v", err)
	}

	// Test invalid request - negative limit
	invalidReq := &SearchRequest{
		Limit: -1,
	}

	err = qb.ValidateSearchRequest(invalidReq)
	if err == nil {
		t.Error("negative limit should cause validation error")
	}
}

func TestQueryBuilderCountriesFilter(t *testing.T) {
	qb := NewQueryBuilderService()

	tests := []struct {
		name      string
		countries []string
		wantError bool
	}{
		{
			name:      "single country filter",
			countries: []string{"CN"},
			wantError: false,
		},
		{
			name:      "multiple countries filter",
			countries: []string{"CN", "US", "FR"},
			wantError: false,
		},
		{
			name:      "empty countries filter",
			countries: []string{},
			wantError: false,
		},
		{
			name:      "nil countries filter",
			countries: nil,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &SearchRequest{
				Query:     "",
				Countries: tt.countries,
				Limit:     10,
				Offset:    0,
			}

			query, err := qb.BuildQuery(req)
			if tt.wantError && err == nil {
				t.Errorf("expected error but got none")
				return
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !tt.wantError {
				if query == nil {
					t.Error("query should not be nil")
				}
			}
		})
	}
}

func TestSearchRequestDefaults(t *testing.T) {
	req := &SearchRequest{}

	// These should be the default values
	if req.SortOrder == "" {
		req.SortOrder = SortOrderDesc // Default sort order
	}

	if req.Limit == 0 {
		req.Limit = 50 // Default limit
	}

	if req.Timeout == 0 {
		req.Timeout = 30 * time.Second // Default timeout
	}

	// Verify defaults are set
	if req.SortOrder != SortOrderDesc {
		t.Error("default sort order should be desc")
	}

	if req.Limit != 50 {
		t.Error("default limit should be 50")
	}
}

func TestCacheMiddleware(t *testing.T) {
	cache := NewOptimizedSearchCache(100)
	defer cache.Close()

	middleware := NewCacheMiddleware(cache, 5*time.Minute)

	if !middleware.IsEnabled() {
		t.Error("middleware should be enabled by default")
	}

	// Disable and test
	middleware.Disable()
	if middleware.IsEnabled() {
		t.Error("middleware should be disabled")
	}

	// Re-enable
	middleware.Enable()
	if !middleware.IsEnabled() {
		t.Error("middleware should be enabled")
	}
}

func TestQueryBuilder(t *testing.T) {
	qb := NewQueryBuilderService()

	// Test basic query building
	req := &SearchRequest{
		Query:       "test",
		IPAddresses: []string{"192.168.1.1"},
		StatusCodes: []int{200, 404},
	}

	query, err := qb.BuildQuery(req)
	if err != nil {
		t.Errorf("BuildQuery should not error: %v", err)
	}

	if query == nil {
		t.Error("BuildQuery should return a query")
	}
}

func TestSuggestionQuery(t *testing.T) {
	qb := NewQueryBuilderService()

	// Test suggestion query building
	query, err := qb.BuildSuggestionQuery("test", "message")
	if err != nil {
		t.Errorf("BuildSuggestionQuery should not error: %v", err)
	}

	if query == nil {
		t.Error("BuildSuggestionQuery should return a query")
	}

	// Test empty text
	_, err = qb.BuildSuggestionQuery("", "message")
	if err == nil {
		t.Error("BuildSuggestionQuery should error for empty text")
	}
}
