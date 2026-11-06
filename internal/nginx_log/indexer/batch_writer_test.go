package indexer

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingIndexer captures every batch a BatchWriter hands over.
type recordingIndexer struct {
	Indexer
	batches [][]*Document
}

func (r *recordingIndexer) IndexDocuments(_ context.Context, docs []*Document) error {
	r.batches = append(r.batches, docs)
	return nil
}

func probeDocument(i int) *Document {
	return &Document{
		ID: "/var/log/nginx/access.log-" + strconv.Itoa(i),
		Fields: &LogDocument{
			IP:          "192.0.2.1",
			Method:      "GET",
			Path:        "/api/v1/products?page=" + strconv.Itoa(i),
			UserAgent:   "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
			Raw:         `192.0.2.1 - - [05/Nov/2026:08:00:00 +0000] "GET /api/v1/products HTTP/1.1" 200 512 "-" "Mozilla/5.0"`,
			FilePath:    "/var/log/nginx/access.log",
			MainLogPath: "/var/log/nginx/access.log",
		},
	}
}

// A batch sized for throughput alone needed more heap than a 1 GiB container
// has; the memory budget has to cut it short first.
func TestBatchWriterFlushesBeforeExceedingMemoryBudget(t *testing.T) {
	perDocument := estimateDocumentBytes(probeDocument(0))
	budget := perDocument*3 + perDocument/2

	sink := &recordingIndexer{}
	writer := NewBatchWriter(sink, 15000, budget)

	for i := range 10 {
		require.NoError(t, writer.Add(probeDocument(i)))
	}
	_, err := writer.Flush()
	require.NoError(t, err)

	var total int
	for _, batch := range sink.batches {
		assert.LessOrEqual(t, estimateDocumentsBytes(batch), budget)
		total += len(batch)
	}
	assert.Equal(t, 10, total)
	assert.Len(t, sink.batches, 4, "3 + 3 + 3 + 1")
}

func TestBatchWriterWithoutMemoryBudgetFlushesOnCount(t *testing.T) {
	sink := &recordingIndexer{}
	writer := NewBatchWriter(sink, 4, 0)

	for i := range 10 {
		require.NoError(t, writer.Add(probeDocument(i)))
	}
	_, err := writer.Flush()
	require.NoError(t, err)

	require.Len(t, sink.batches, 3)
	assert.Len(t, sink.batches[0], 4)
	assert.Len(t, sink.batches[2], 2)
}

// The indexer refuses a job larger than its whole quota instead of waiting
// for room, so the batches it is given must already fit inside it.
func TestIndexingFileStaysWithinMemoryQuota(t *testing.T) {
	logDir := t.TempDir()
	allowLogDir(t, logDir)
	logPath := filepath.Join(logDir, "access.log")
	require.NoError(t, os.WriteFile(logPath, []byte(accessLogLines(0, 200)), 0o600))

	config := DefaultIndexerConfig()
	config.IndexPath = t.TempDir()
	config.BatchSize = 15000
	config.FileGroupConcurrency = 2
	// Room for a few dozen documents: a 15000-document batch would be refused.
	config.MemoryQuota = estimateDocumentBytes(probeDocument(0)) * 40

	pi := NewParallelIndexer(config, NewGroupedShardManager(config))
	require.NoError(t, pi.Start(context.Background()))
	t.Cleanup(func() { _ = pi.Stop() })

	assert.Equal(t, config.MemoryQuota/2, pi.batchMemoryBudget(), "each concurrent file gets an equal share")

	docs, _, _, err := pi.IndexSingleFileWithProgress(logPath, nil)
	require.NoError(t, err)
	assert.EqualValues(t, 200, docs)

	count, err := pi.CountDocsByFilePath(logPath)
	require.NoError(t, err)
	assert.EqualValues(t, 200, count)
	assert.Zero(t, pi.memoryLimit.usage(), "every reservation is released")
}
