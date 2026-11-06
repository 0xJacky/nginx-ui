package indexer

import (
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/logstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestColumnarIndexer(t *testing.T) (*ColumnarIndexer, string) {
	t.Helper()
	InitLogParser()
	logDir := t.TempDir()
	allowLogDir(t, logDir)
	config := DefaultIndexerConfig()
	config.IndexPath = t.TempDir()
	ci := NewColumnarIndexer(config)
	require.NoError(t, ci.Start(context.Background()))
	t.Cleanup(func() { _ = ci.Stop() })
	return ci, logDir
}

func writeGzip(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	zw := gzip.NewWriter(f)
	_, err = zw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())
}

// A log group's live, rotated and compressed files each get their own
// segment under the group, and the counts and deletes follow that layout.
func TestColumnarIndexerIndexesAndDeletesLogGroups(t *testing.T) {
	ci, logDir := newTestColumnarIndexer(t)
	base := filepath.Join(logDir, "access.log")
	require.NoError(t, os.WriteFile(base, []byte(accessLogLines(0, 30)), 0o600))
	require.NoError(t, os.WriteFile(base+".1", []byte(accessLogLines(100, 20)), 0o600))
	writeGzip(t, base+".2.gz", accessLogLines(200, 10))
	other := filepath.Join(logDir, "api.log")
	require.NoError(t, os.WriteFile(other, []byte(accessLogLines(300, 5)), 0o600))

	counts, minTime, maxTime, err := ci.IndexLogGroupWithProgress(base, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]uint64{base: 30, base + ".1": 20, base + ".2.gz": 10}, counts)
	require.NotNil(t, minTime)
	require.NotNil(t, maxTime)
	_, _, _, err = ci.IndexSingleFile(other)
	require.NoError(t, err)

	group, err := ci.CountDocsByMainLogPath(base)
	require.NoError(t, err)
	assert.EqualValues(t, 60, group)
	rotated, err := ci.CountDocsByFilePath(base + ".1")
	require.NoError(t, err)
	assert.EqualValues(t, 20, rotated)

	// Plain files record how far they were read; compressed ones do not.
	info, err := os.Stat(base)
	require.NoError(t, err)
	pos, ok := ci.takeReadPosition(base)
	require.True(t, ok)
	assert.Equal(t, info.Size(), pos)
	assert.Equal(t, info.Size(), ci.Store().Segment(base).SourceOffset())
	_, ok = ci.takeReadPosition(base + ".2.gz")
	assert.False(t, ok)

	stats := ci.GetStats()
	assert.EqualValues(t, 65, stats.TotalDocuments)
	assert.Equal(t, 4, stats.ShardCount)
	assert.Positive(t, stats.TotalSize)

	require.NoError(t, ci.DeleteIndexByFiles([]string{base + ".1"}))
	group, err = ci.CountDocsByMainLogPath(base)
	require.NoError(t, err)
	assert.EqualValues(t, 40, group)

	require.NoError(t, ci.DeleteIndexByMainLogPath(base))
	group, err = ci.CountDocsByMainLogPath(base)
	require.NoError(t, err)
	assert.Zero(t, group)
	remaining, err := ci.CountDocsByFilePath(other)
	require.NoError(t, err)
	assert.EqualValues(t, 5, remaining, "other groups are untouched")

	// A full rebuild destroys everything and starts again on the same store.
	require.NoError(t, ci.DestroyAllIndexes(context.Background()))
	require.NoError(t, ci.Start(context.Background()))
	assert.Zero(t, ci.GetStats().TotalDocuments)
	_, _, _, err = ci.IndexSingleFile(other)
	require.NoError(t, err)
	assert.EqualValues(t, 5, ci.GetStats().TotalDocuments)
}

// A read that fails after some blocks were sealed leaves nothing behind, so
// the retry from the same position cannot duplicate rows.
func TestColumnarIndexerFailedReadLeavesNoRows(t *testing.T) {
	ci, logDir := newTestColumnarIndexer(t)
	logPath := filepath.Join(logDir, "access.log")

	lines := logstore.DefaultBlockRows + 500
	content := accessLogLines(0, lines) + strings.Repeat("x", 64*1024) + "\n"
	require.NoError(t, os.WriteFile(logPath, []byte(content), 0o600))

	_, _, _, err := ci.IndexSingleFile(logPath)
	require.Error(t, err, "a line over the parser's limit fails the read")

	count, err := ci.CountDocsByFilePath(logPath)
	require.NoError(t, err)
	assert.Zero(t, count, "the block sealed before the failure was rolled back")
	_, ok := ci.takeReadPosition(logPath)
	assert.False(t, ok, "a failed read records no position")
}

func TestColumnarIndexerStoppedRefusesWork(t *testing.T) {
	ci, logDir := newTestColumnarIndexer(t)
	require.NoError(t, ci.Stop())
	assert.False(t, ci.IsHealthy())
	_, _, _, err := ci.IndexSingleFile(filepath.Join(logDir, "access.log"))
	assert.Error(t, err)
	_, err = ci.CountDocsByMainLogPath("/var/log/nginx/access.log")
	assert.Error(t, err)
	assert.Error(t, ci.Stop())
	require.NoError(t, ci.Start(context.Background()), "a stopped indexer can start again")
}
