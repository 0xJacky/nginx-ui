package searcher

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newStatsTestSearcher builds a single-shard searcher holding docCount
// documents whose bytes_sent is 100, 200, 300, ... and whose request_time is
// 0.1, 0.2, 0.3, ... so the expected sums are closed-form.
func newStatsTestSearcher(t *testing.T, docCount int) *Searcher {
	t.Helper()

	shardPath := filepath.Join(t.TempDir(), "stats.bleve")
	shard, err := bleve.New(shardPath, bleve.NewIndexMapping())
	require.NoError(t, err)
	t.Cleanup(func() { _ = shard.Close() })

	batch := shard.NewBatch()
	for i := 1; i <= docCount; i++ {
		doc := map[string]interface{}{
			"timestamp":    float64(1700000000 + i),
			"bytes_sent":   float64(i * 100),
			"request_time": float64(i) / 10,
		}
		require.NoError(t, batch.Index(strconv.Itoa(i), doc))
	}
	require.NoError(t, shard.Batch(batch))

	searcher := NewSearcher(DefaultSearcherConfig(), []bleve.Index{shard})
	t.Cleanup(func() { _ = searcher.Stop() })

	return searcher
}

func TestSearcher_IncludeStatsSumsWholeMatchSet(t *testing.T) {
	const docCount = 120
	searcher := newStatsTestSearcher(t, docCount)

	// A page of 10 hits must not limit the totals: the stats scan covers every
	// matching document, which is the whole point of the aggregation.
	result, err := searcher.Search(context.Background(), &SearchRequest{
		Limit:        10,
		IncludeStats: true,
		UseCache:     false,
	})
	require.NoError(t, err)
	require.NotNil(t, result.Stats)
	assert.Len(t, result.Hits, 10)
	assert.Equal(t, uint64(docCount), result.TotalHits)

	// 100 + 200 + ... + 12000
	expectedBytes := int64(100 * docCount * (docCount + 1) / 2)
	assert.Equal(t, expectedBytes, result.Stats.TotalBytes)
	assert.Equal(t, uint64(docCount), result.Stats.ScannedDocs)
	assert.False(t, result.Stats.Approximate)

	assert.InDelta(t, float64(expectedBytes)/docCount, result.Stats.AvgBytes, 0.001)
	assert.Equal(t, int64(100), result.Stats.MinBytes)
	assert.Equal(t, int64(docCount*100), result.Stats.MaxBytes)

	assert.InDelta(t, 0.1, result.Stats.MinReqTime, 0.0001)
	assert.InDelta(t, float64(docCount)/10, result.Stats.MaxReqTime, 0.0001)
}

func TestSearcher_IncludeStatsRespectsFilters(t *testing.T) {
	searcher := newStatsTestSearcher(t, 10)

	// The time range is inclusive of start and exclusive of end, so this picks
	// documents 3, 4 and 5, summing to 300 + 400 + 500.
	start := int64(1700000003)
	end := int64(1700000006)
	result, err := searcher.Search(context.Background(), &SearchRequest{
		Limit:        10,
		StartTime:    &start,
		EndTime:      &end,
		IncludeStats: true,
		UseCache:     false,
	})
	require.NoError(t, err)
	require.NotNil(t, result.Stats)

	assert.Equal(t, uint64(3), result.TotalHits)
	assert.Equal(t, int64(1200), result.Stats.TotalBytes)
	assert.False(t, result.Stats.Approximate)
}

func TestSearcher_IncludeStatsOnEmptyResult(t *testing.T) {
	searcher := newStatsTestSearcher(t, 5)

	// A range with no documents must yield zeroed stats rather than the
	// sentinel values the min/max tracking starts from.
	start := int64(1800000000)
	end := int64(1800000100)
	result, err := searcher.Search(context.Background(), &SearchRequest{
		Limit:        10,
		StartTime:    &start,
		EndTime:      &end,
		IncludeStats: true,
		UseCache:     false,
	})
	require.NoError(t, err)
	require.NotNil(t, result.Stats)

	assert.Equal(t, uint64(0), result.TotalHits)
	assert.Equal(t, int64(0), result.Stats.TotalBytes)
	assert.Equal(t, int64(0), result.Stats.MinBytes)
	assert.InDelta(t, 0.0, result.Stats.MinReqTime, 0.0001)
	assert.False(t, result.Stats.Approximate)
}

func TestCache_StatsKeyIgnoresPagination(t *testing.T) {
	cache := NewCache(100)
	t.Cleanup(cache.Close)

	start := int64(1700000000)
	base := &SearchRequest{
		Query:        "GET",
		StartTime:    &start,
		LogPaths:     []string{"/var/log/nginx/access.log"},
		Limit:        50,
		IncludeStats: true,
		UseCache:     true,
	}

	// Paging or re-sorting does not change which documents match, so the
	// aggregation must be shared rather than recomputed.
	nextPage := *base
	nextPage.Offset = 50
	nextPage.SortBy = "status"
	nextPage.SortOrder = "asc"
	nextPage.IncludeFacets = true
	assert.Equal(t, cache.statsCacheKey(base), cache.statsCacheKey(&nextPage))

	// Changing a filter does change the match set.
	narrowed := *base
	narrowed.StatusCodes = []int{500}
	assert.NotEqual(t, cache.statsCacheKey(base), cache.statsCacheKey(&narrowed))
}

func TestCache_SearchStatsRoundTrip(t *testing.T) {
	cache := NewCache(100)
	t.Cleanup(cache.Close)

	req := &SearchRequest{Query: "GET", IncludeStats: true, UseCache: true}
	assert.Nil(t, cache.GetSearchStats(req), "nothing is cached yet")

	cache.PutSearchStats(req, &SearchStats{TotalBytes: 4096, ScannedDocs: 8}, time.Minute)

	cached := cache.GetSearchStats(req)
	require.NotNil(t, cached)
	assert.Equal(t, int64(4096), cached.TotalBytes)
	assert.Equal(t, uint64(8), cached.ScannedDocs)
}

func TestSearcher_StatsOmittedWhenNotRequested(t *testing.T) {
	searcher := newStatsTestSearcher(t, 5)

	result, err := searcher.Search(context.Background(), &SearchRequest{
		Limit:    10,
		UseCache: false,
	})
	require.NoError(t, err)
	assert.Nil(t, result.Stats, "the scan must not run unless the caller asks for stats")
}
