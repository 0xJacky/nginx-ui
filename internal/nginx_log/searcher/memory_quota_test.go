package searcher

import (
	"context"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearcherRejectsQueryOverMemoryQuota(t *testing.T) {
	index, err := bleve.NewMemOnly(bleve.NewIndexMapping())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })
	require.NoError(t, index.Index("one", map[string]interface{}{"message": "hello"}))

	config := DefaultSearcherConfig()
	config.EnableCache = false
	config.MemoryQuota = 1
	searcher := NewSearcher(config, []bleve.Index{index})
	t.Cleanup(func() { require.NoError(t, searcher.Stop()) })

	_, err = searcher.Search(context.Background(), &SearchRequest{Query: "hello", Limit: 10})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "search memory quota")
}
