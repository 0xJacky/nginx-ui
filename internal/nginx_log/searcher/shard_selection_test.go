package searcher

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingIndexDelegate interface {
	bleve.Index
}

type countingSearchIndex struct {
	countingIndexDelegate
	searches atomic.Int64
}

func (index *countingSearchIndex) SearchInContext(
	ctx context.Context,
	request *bleve.SearchRequest,
) (*bleve.SearchResult, error) {
	index.searches.Add(1)
	return index.countingIndexDelegate.SearchInContext(ctx, request)
}

func TestSearcherNarrowsMainLogPathQueriesToMatchingShards(t *testing.T) {
	first := newNamedCountingIndex(t, "/var/log/nginx/access.log")
	second := newNamedCountingIndex(t, "/var/log/nginx/other.log")

	searcher := NewSearcher(DefaultSearcherConfig(), []bleve.Index{first, second})
	t.Cleanup(func() {
		require.NoError(t, searcher.Stop())
	})

	result, err := searcher.Search(context.Background(), &SearchRequest{
		UseMainLogPath: true,
		LogPaths:       []string{"/var/log/nginx/access.log"},
		Limit:          10,
	})
	require.NoError(t, err)
	require.Len(t, result.Hits, 1)
	assert.Greater(t, first.searches.Load(), int64(0))
	assert.Zero(t, second.searches.Load())
}

func TestCounterNarrowsMainLogPathQueriesToMatchingShards(t *testing.T) {
	first := newNamedCountingIndex(t, "/var/log/nginx/access.log")
	second := newNamedCountingIndex(t, "/var/log/nginx/other.log")

	counter := NewCounter([]bleve.Index{first, second})
	t.Cleanup(func() {
		require.NoError(t, counter.Stop())
	})

	result, err := counter.Count(context.Background(), &CardinalityRequest{
		Field:          "method",
		UseMainLogPath: true,
		LogPaths:       []string{"/var/log/nginx/access.log"},
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result.Cardinality)
	assert.Greater(t, first.searches.Load(), int64(0))
	assert.Zero(t, second.searches.Load())
}

func TestSearcherNarrowsDashboardQueriesAcrossMultipleLogGroups(t *testing.T) {
	const (
		firstPath  = "/var/log/nginx/access_1.log"
		secondPath = "/var/log/nginx/access_2.log"
		thirdPath  = "/var/log/nginx/access_3.log"
	)

	shards := []bleve.Index{
		newNamedCountingIndexAt(t, firstPath, "first-1", 100),
		newNamedCountingIndexAt(t, firstPath, "first-2", 101),
		newNamedCountingIndexAt(t, secondPath, "second-1", 200),
		newNamedCountingIndexAt(t, secondPath, "second-2", 201),
		newNamedCountingIndexAt(t, thirdPath, "third-1", 300),
		newNamedCountingIndexAt(t, thirdPath, "third-2", 301),
	}
	searcher := NewSearcher(DefaultSearcherConfig(), shards)
	t.Cleanup(func() {
		require.NoError(t, searcher.Stop())
	})

	tests := []struct {
		path  string
		start int64
		end   int64
	}{
		{path: firstPath, start: 100, end: 102},
		{path: secondPath, start: 200, end: 202},
		{path: thirdPath, start: 300, end: 302},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			for attempt := 0; attempt < 2; attempt++ {
				result, err := searcher.Search(context.Background(), &SearchRequest{
					UseMainLogPath: true,
					LogPaths:       []string{tt.path},
					StartTime:      &tt.start,
					EndTime:        &tt.end,
					IncludeFacets:  true,
					FacetFields:    []string{"method"},
					UseCache:       true,
					Limit:          -1,
				})
				require.NoError(t, err)
				assert.Equal(t, uint64(2), result.TotalHits)
			}
		})
	}
}

func newNamedCountingIndex(t *testing.T, mainLogPath string) *countingSearchIndex {
	return newNamedCountingIndexAt(t, mainLogPath, mainLogPath, 1)
}

func newNamedCountingIndexAt(t *testing.T, mainLogPath, id string, timestamp int64) *countingSearchIndex {
	t.Helper()

	index, err := bleve.NewMemOnly(indexer.CreateLogIndexMapping())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, index.Close())
	})
	index.SetName(mainLogPath)
	require.NoError(t, index.Index(id, map[string]interface{}{
		"timestamp":     timestamp,
		"method":        "GET",
		"main_log_path": mainLogPath,
		"file_path":     mainLogPath,
		"raw":           "request",
	}))

	return &countingSearchIndex{countingIndexDelegate: index}
}
