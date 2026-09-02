package indexer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParallelIndexerRoutesDocumentsToTheirOwnLogGroupShards(t *testing.T) {
	config := DefaultIndexerConfig()
	config.IndexPath = t.TempDir()
	config.ShardCount = 2
	config.WorkerCount = 1
	config.MaxQueueSize = 4

	manager := NewGroupedShardManager(config)
	parallelIndexer := NewParallelIndexer(config, manager)
	require.NoError(t, parallelIndexer.Start(context.Background()))
	t.Cleanup(func() {
		require.NoError(t, parallelIndexer.Stop())
	})

	documents := []*Document{
		newGroupedRoutingDocument("first", "/var/log/nginx/access_1.log"),
		newGroupedRoutingDocument("second", "/var/log/nginx/access_2.log"),
	}
	require.NoError(t, parallelIndexer.IndexDocuments(context.Background(), documents))

	documentsByGroup := make(map[string]uint64)
	for _, shard := range parallelIndexer.GetAllShards() {
		count, err := shard.DocCount()
		require.NoError(t, err)
		documentsByGroup[shard.Name()] += count
	}

	assert.Equal(t, uint64(1), documentsByGroup["/var/log/nginx/access_1.log"])
	assert.Equal(t, uint64(1), documentsByGroup["/var/log/nginx/access_2.log"])
}

func newGroupedRoutingDocument(id, mainLogPath string) *Document {
	return &Document{
		ID: id,
		Fields: &LogDocument{
			Timestamp:   1,
			Method:      "GET",
			Path:        "/",
			PathExact:   "/",
			Status:      200,
			FilePath:    mainLogPath,
			MainLogPath: mainLogPath,
			Raw:         "request",
		},
	}
}
