package indexer

import (
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

func TestOpenExistingShardReturnsWhenIndexIsLocked(t *testing.T) {
	shardPath := t.TempDir()
	lockedIndex, err := bleve.New(shardPath, CreateLogIndexMapping())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, lockedIndex.Close())
	})

	startedAt := time.Now()
	_, err = openExistingShard(shardPath, 50*time.Millisecond)

	require.ErrorIs(t, err, bolt.ErrTimeout)
	require.Less(t, time.Since(startedAt), time.Second)
}
