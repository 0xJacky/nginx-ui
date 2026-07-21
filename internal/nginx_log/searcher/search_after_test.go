package searcher

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/require"
)

// TestSearchAfterPagination verifies cursor-based pagination over documents
// that share identical timestamps. Nginx timestamps have 1-second resolution,
// so a stable pagination cursor must not skip or duplicate documents within
// the same second — this relies on the _id sort tiebreaker.
func TestSearchAfterPagination(t *testing.T) {
	index, err := bleve.NewMemOnly(indexer.CreateLogIndexMapping())
	require.NoError(t, err)
	t.Cleanup(func() { _ = index.Close() })

	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC).Unix()
	const totalDocs = 25

	// Three distinct seconds, many documents sharing each second
	for i := 0; i < totalDocs; i++ {
		id := fmt.Sprintf("doc-%02d", i)
		require.NoError(t, index.Index(id, map[string]interface{}{
			"timestamp":     baseTime + int64(i%3),
			"ip":            fmt.Sprintf("10.0.0.%d", i),
			"method":        "GET",
			"status":        200,
			"path":          "/p",
			"path_exact":    "/p",
			"file_path":     "/var/log/nginx/access.log",
			"main_log_path": "/var/log/nginx/access.log",
			"raw":           id,
		}))
	}

	config := DefaultSearcherConfig()
	config.EnableCache = false
	s := NewSearcher(config, []bleve.Index{index})
	defer func() { _ = s.Stop() }()

	const pageSize = 10
	seen := make(map[string]bool)
	var searchAfter []string
	pages := 0

	for {
		result, err := s.Search(context.Background(), &SearchRequest{
			Limit:       pageSize,
			SearchAfter: searchAfter,
			SortBy:      "timestamp",
			SortOrder:   "asc",
		})
		require.NoError(t, err)

		for _, hit := range result.Hits {
			if seen[hit.ID] {
				t.Fatalf("document %s returned twice across pages", hit.ID)
			}
			seen[hit.ID] = true
		}

		pages++
		if len(result.Hits) < pageSize {
			break
		}

		lastHit := result.Hits[len(result.Hits)-1]
		require.NotEmpty(t, lastHit.Sort, "hits must carry sort values for cursor pagination")
		searchAfter = lastHit.Sort

		if pages > totalDocs {
			t.Fatal("pagination did not terminate")
		}
	}

	if len(seen) != totalDocs {
		t.Errorf("collected %d unique documents, want %d", len(seen), totalDocs)
	}
	if pages != 3 {
		t.Errorf("pages = %d, want 3 (10+10+5)", pages)
	}
}
