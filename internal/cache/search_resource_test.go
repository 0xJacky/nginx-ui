package cache

import (
	"context"
	"strings"
	"testing"

	"github.com/blevesearch/bleve/v2/mapping"
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

func TestSearchIndexerTracksUpdatesAndDeletes(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1024)

	document := SearchDocument{ID: "example", Content: "server {}"}
	require.NoError(t, indexer.IndexDocument(document))

	document.Content = strings.Repeat("x", 32)
	require.NoError(t, indexer.IndexDocument(document))
	totalBytes, documentCount, _ := indexer.getMemoryUsage()
	assert.Equal(t, int64(32), totalBytes)
	assert.Equal(t, int64(1), documentCount)

	require.NoError(t, indexer.DeleteDocument(document.ID))
	totalBytes, documentCount, _ = indexer.getMemoryUsage()
	assert.Zero(t, totalBytes)
	assert.Zero(t, documentCount)

	count, err := indexer.index.DocCount()
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestHandleConfigScanDeletesRemovedConfig(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1024)
	configPath := "/etc/nginx/sites-enabled/example.conf"

	require.NoError(t, indexer.handleConfigScan(configPath, []byte("server { listen 80; }")))
	require.NoError(t, indexer.handleConfigScan(configPath, nil))

	count, err := indexer.index.DocCount()
	require.NoError(t, err)
	assert.Zero(t, count)
	totalBytes, documentCount, _ := indexer.getMemoryUsage()
	assert.Zero(t, totalBytes)
	assert.Zero(t, documentCount)
}

func TestSearchIndexerRebuildResetsAccounting(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1024)
	require.NoError(t, indexer.IndexDocument(SearchDocument{ID: "example", Content: "server {}"}))

	require.NoError(t, indexer.RebuildIndex(context.Background()))

	totalBytes, documentCount, _ := indexer.getMemoryUsage()
	assert.Zero(t, totalBytes)
	assert.Zero(t, documentCount)
}

func TestSearchIndexMappingDisablesUnusedIndexFeatures(t *testing.T) {
	indexer := &SearchIndexer{}
	indexMapping, ok := indexer.createIndexMapping().(*mapping.IndexMappingImpl)
	require.True(t, ok)
	assert.False(t, indexMapping.IndexDynamic)
	assert.False(t, indexMapping.StoreDynamic)
	assert.False(t, indexMapping.DocValuesDynamic)

	documentMapping := indexMapping.DefaultMapping
	require.NotNil(t, documentMapping)
	assert.False(t, documentMapping.Dynamic)

	tests := []struct {
		name  string
		store bool
		index bool
	}{
		{name: "id", store: false, index: false},
		{name: "type", store: true, index: true},
		{name: "path", store: true, index: false},
		{name: "name", store: true, index: true},
		{name: "content", store: true, index: true},
		{name: "updated_at", store: true, index: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			property := documentMapping.Properties[tt.name]
			require.NotNil(t, property)
			require.Len(t, property.Fields, 1)
			field := property.Fields[0]
			assert.Equal(t, tt.store, field.Store)
			assert.Equal(t, tt.index, field.Index)
			assert.False(t, field.DocValues)
			assert.False(t, field.IncludeTermVectors)
			assert.False(t, field.IncludeInAll)
		})
	}
}
