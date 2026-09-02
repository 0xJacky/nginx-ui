package indexer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/index/scorch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newResourceTestShardManager(t *testing.T) (*GroupedShardManager, bleve.Index) {
	t.Helper()

	shardPath := filepath.Join(t.TempDir(), "shard_0")
	shard, err := bleve.New(shardPath, CreateLogIndexMapping())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, shard.Close())
	})

	manager := NewGroupedShardManager(&Config{IndexPath: filepath.Dir(shardPath), ShardCount: 1})
	manager.groups["group"] = &ShardGroup{
		UUID:        "group",
		MainLogPath: "/var/log/nginx/access.log",
		Shards:      map[int]bleve.Index{0: shard},
		ShardPaths:  map[int]string{0: shardPath},
		ShardCount:  1,
	}
	manager.globalToLocal[0] = groupShardRef{uuid: "group", localID: 0}
	manager.localToGlobal[manager.makeLocalKey("group", 0)] = 0
	return manager, shard
}

func TestGroupedShardStatsIncludeFilesInsideShardDirectory(t *testing.T) {
	manager, _ := newResourceTestShardManager(t)
	shardPath := manager.groups["group"].ShardPaths[0]
	marker := make([]byte, 32*1024)
	require.NoError(t, os.WriteFile(filepath.Join(shardPath, "size-marker"), marker, 0o600))

	stats := manager.GetShardStats()
	require.Len(t, stats, 1)
	assert.GreaterOrEqual(t, stats[0].Size, int64(len(marker)))
}

func TestOptimizeShardUsesScorchForceMerge(t *testing.T) {
	manager, shard := newResourceTestShardManager(t)
	for batchNumber := 0; batchNumber < 3; batchNumber++ {
		batch := shard.NewBatch()
		batch.Index(string(rune('a'+batchNumber)), map[string]any{"raw": "request"})
		require.NoError(t, shard.Batch(batch))
	}

	before := scorchStat(t, shard, "TotFileMergeForceOpsCompleted")
	require.NoError(t, manager.OptimizeShard(0))
	after := scorchStat(t, shard, "TotFileMergeForceOpsCompleted")
	assert.Greater(t, after, before)

	value, err := shard.GetInternal([]byte("_optimize"))
	require.NoError(t, err)
	assert.Empty(t, value)
}

func TestFlushAllDoesNotCreateSyntheticMutations(t *testing.T) {
	manager, shard := newResourceTestShardManager(t)
	config := DefaultIndexerConfig()
	config.OptimizeInterval = 0
	indexer := NewParallelIndexer(config, manager)
	atomic.StoreInt32(&indexer.running, 1)

	beforeBatches := scorchStat(t, shard, "TotBatches")
	beforeDeletes := scorchStat(t, shard, "TotDeletes")
	require.NoError(t, indexer.FlushAll())
	assert.Equal(t, beforeBatches, scorchStat(t, shard, "TotBatches"))
	assert.Equal(t, beforeDeletes, scorchStat(t, shard, "TotDeletes"))
}

func TestIndexerRejectsJobLargerThanMemoryQuota(t *testing.T) {
	config := DefaultIndexerConfig()
	config.IndexPath = t.TempDir()
	config.ShardCount = 1
	config.WorkerCount = 1
	config.MaxQueueSize = 1
	config.MemoryQuota = 128
	config.OptimizeInterval = 0
	config.EnableMetrics = false

	indexer := NewParallelIndexer(config, NewGroupedShardManager(config))
	require.NoError(t, indexer.Start(context.Background()))
	t.Cleanup(func() {
		require.NoError(t, indexer.Stop())
	})

	err := indexer.IndexDocument(context.Background(), &Document{
		ID: "oversized",
		Fields: &LogDocument{
			MainLogPath: "/var/log/nginx/access.log",
			FilePath:    "/var/log/nginx/access.log",
			Raw:         strings.Repeat("x", 1024),
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "memory quota")
}

func TestDefaultIndexerResourceBounds(t *testing.T) {
	config := DefaultIndexerConfig()
	assert.LessOrEqual(t, config.ShardCount, 2)
	assert.LessOrEqual(t, config.MaxQueueSize, config.WorkerCount*4)
}

func TestIndexerProfilesBoundQueueDepthByWorkerCount(t *testing.T) {
	for _, profile := range []string{
		"high_throughput",
		"low_latency",
		"balanced",
		"memory_constrained",
		"cpu_intensive",
		"max_performance",
	} {
		t.Run(profile, func(t *testing.T) {
			config := GetConfig(profile)
			assert.GreaterOrEqual(t, config.MaxQueueSize, config.WorkerCount)
			assert.LessOrEqual(t, config.MaxQueueSize, config.WorkerCount*4)
		})
	}
}

func TestScorchRuntimeConfigUsesSegmentMemoryLimit(t *testing.T) {
	config := &Config{MaxSegmentSize: 60 * 1024 * 1024}
	runtimeConfig := scorchRuntimeConfig(config)

	persister, ok := runtimeConfig["scorchPersisterOptions"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 1, persister["NumPersisterWorkers"])
	assert.Equal(t, int(config.MaxSegmentSize), persister["MaxSizeInMemoryMergePerWorker"])

	mergePlan, ok := runtimeConfig["scorchMergePlanOptions"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, config.MaxSegmentSize/6, mergePlan["FloorSegmentFileSize"])
}

func scorchStat(t *testing.T, index bleve.Index, name string) uint64 {
	t.Helper()

	advanced, err := index.Advanced()
	require.NoError(t, err)
	scorchIndex, ok := advanced.(*scorch.Scorch)
	require.True(t, ok, "advanced index has type %T", advanced)
	value, ok := scorchIndex.StatsMap()[name]
	require.True(t, ok, "missing Scorch stat %q", name)
	stat, ok := value.(uint64)
	require.True(t, ok, "Scorch stat %q has type %T", name, value)
	return stat
}
