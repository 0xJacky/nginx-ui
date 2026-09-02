package searcher

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchCacheRejectsResultLargerThanByteBudget(t *testing.T) {
	cache := NewCache(100)
	t.Cleanup(cache.Close)

	request := &SearchRequest{Query: "large", Limit: 1}
	result := &SearchResult{Hits: []*SearchHit{{
		ID:     "large",
		Fields: map[string]interface{}{"raw": strings.Repeat("x", 256*1024)},
	}}}

	cache.Put(request, result, time.Minute)
	assert.Nil(t, cache.Get(request))
}

func TestSearchCacheStoresResultWithinByteBudget(t *testing.T) {
	cache := NewCache(100)
	t.Cleanup(cache.Close)

	request := &SearchRequest{Query: "small", Limit: 1}
	result := &SearchResult{Hits: []*SearchHit{{
		ID:     "small",
		Fields: map[string]interface{}{"status": 200},
	}}}

	cache.Put(request, result, time.Minute)
	cached := cache.Get(request)
	require.NotNil(t, cached)
	assert.True(t, cached.FromCache)
}
