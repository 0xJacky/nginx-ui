package cache

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSearchIndexer(t *testing.T, maxContentBytes int64) *SearchIndexer {
	t.Helper()

	indexer := &SearchIndexer{
		indexPath:      t.TempDir(),
		maxMemoryUsage: maxContentBytes,
	}
	require.NoError(t, indexer.Initialize(context.Background()))
	t.Cleanup(func() {
		require.NoError(t, indexer.Close())
	})
	return indexer
}

func documentCount(t *testing.T, indexer *SearchIndexer) int {
	t.Helper()

	stats, err := indexer.GetIndexStats()
	require.NoError(t, err)
	return int(stats["document_count"].(int64))
}

func TestSearchIndexerTracksUpdatesAndDeletes(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1024)

	document := SearchDocument{ID: "example", Content: "server {}"}
	require.NoError(t, indexer.IndexDocument(document))

	document.Content = strings.Repeat("x", 32)
	require.NoError(t, indexer.IndexDocument(document))
	totalBytes, count, _ := indexer.getMemoryUsage()
	assert.Equal(t, int64(32), totalBytes)
	assert.Equal(t, int64(1), count)

	require.NoError(t, indexer.DeleteDocument(document.ID))
	totalBytes, count, _ = indexer.getMemoryUsage()
	assert.Zero(t, totalBytes)
	assert.Zero(t, count)
	assert.Zero(t, documentCount(t, indexer))
}

func TestHandleConfigScanDeletesRemovedConfig(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1024)
	configPath := "/etc/nginx/sites-enabled/example.conf"

	require.NoError(t, indexer.handleConfigScan(configPath, []byte("server { listen 80; }")))
	require.Equal(t, 1, documentCount(t, indexer))
	require.NoError(t, indexer.handleConfigScan(configPath, nil))

	assert.Zero(t, documentCount(t, indexer))
	totalBytes, count, _ := indexer.getMemoryUsage()
	assert.Zero(t, totalBytes)
	assert.Zero(t, count)
}

func TestSearchIndexerRebuildResetsAccounting(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1024)
	require.NoError(t, indexer.IndexDocument(SearchDocument{ID: "example", Content: "server {}"}))

	require.NoError(t, indexer.RebuildIndex(context.Background()))

	totalBytes, count, _ := indexer.getMemoryUsage()
	assert.Zero(t, totalBytes)
	assert.Zero(t, count)
	assert.Zero(t, documentCount(t, indexer))
}

func TestSearchIndexerSkipsDocumentsOverContentBudget(t *testing.T) {
	indexer := newTestSearchIndexer(t, 64)

	require.NoError(t, indexer.IndexDocument(SearchDocument{ID: "small", Name: "small", Content: "server {}"}))
	require.NoError(t, indexer.IndexDocument(SearchDocument{ID: "big", Name: "big", Content: strings.Repeat("x", 128)}))

	assert.Equal(t, 1, documentCount(t, indexer))
	results, err := indexer.Search(context.Background(), "big", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchIndexerRejectsOversizedDocuments(t *testing.T) {
	indexer := newTestSearchIndexer(t, 10*1024*1024)

	err := indexer.IndexDocument(SearchDocument{ID: "huge", Content: strings.Repeat("x", maxDocumentContentSize+1)})
	require.Error(t, err)
	assert.Zero(t, documentCount(t, indexer))
}

func TestSearchIndexerIndexStats(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1024)
	require.NoError(t, indexer.IndexDocument(SearchDocument{ID: "example", Content: "server {}"}))

	stats, err := indexer.GetIndexStats()
	require.NoError(t, err)
	for _, key := range []string{"document_count", "tracked_document_count", "total_content_size", "max_memory_usage", "memory_usage_percent", "index_path"} {
		assert.Contains(t, stats, key)
	}
	assert.Equal(t, int64(1), stats["document_count"])
	assert.Equal(t, int64(9), stats["total_content_size"])
	assert.Equal(t, int64(1024), stats["max_memory_usage"])
}

func TestSearchIndexerUninitializedAndClosed(t *testing.T) {
	indexer := &SearchIndexer{}
	ctx := context.Background()

	_, err := indexer.Search(ctx, "example", 10)
	require.Error(t, err)
	require.Error(t, indexer.IndexDocument(SearchDocument{ID: "example"}))
	require.Error(t, indexer.DeleteDocument("example"))
	_, err = indexer.GetIndexStats()
	require.Error(t, err)

	require.NoError(t, indexer.Initialize(ctx))
	require.NoError(t, indexer.IndexDocument(SearchDocument{ID: "example", Name: "example", Content: "server {}"}))
	require.NoError(t, indexer.Close())

	_, err = indexer.Search(ctx, "example", 10)
	require.Error(t, err)
	// Close is idempotent
	require.NoError(t, indexer.Close())
}
