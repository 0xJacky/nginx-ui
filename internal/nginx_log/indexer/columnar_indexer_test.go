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

// syncGroup indexes a log group the way the scheduler and the cron job do and
// returns the rows stored per current path.
func syncGroup(t *testing.T, ci *ColumnarIndexer, base string) map[string]int64 {
	t.Helper()
	_, _, _, err := ci.IndexLogGroupWithProgress(base, nil)
	require.NoError(t, err)
	rows := map[string]int64{}
	for _, seg := range ci.Store().Segments(base) {
		rows[seg.Path()] += seg.Info().Rows
	}
	return rows
}

// logrotate renames access.log to access.log.1, later compresses it, and
// nginx starts a new access.log. The rotated content keeps its rows and only
// lines that are new are read.
func TestColumnarIndexerFollowsRotatedContent(t *testing.T) {
	ci, logDir := newTestColumnarIndexer(t)
	base := filepath.Join(logDir, "access.log")

	require.NoError(t, os.WriteFile(base, []byte(accessLogLines(0, 20)), 0o600))
	assert.Equal(t, map[string]int64{base: 20}, syncGroup(t, ci, base))
	before := ci.docsIndexed.Load()

	// Rename and start a new file; nginx writes two more lines to the
	// renamed file before reopening, as it does until it gets USR1.
	require.NoError(t, os.Rename(base, base+".1"))
	f, err := os.OpenFile(base+".1", os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteString(accessLogLines(20, 2))
	require.NoError(t, err)
	require.NoError(t, f.Close())
	require.NoError(t, os.WriteFile(base, []byte(accessLogLines(100, 5)), 0o600))

	assert.Equal(t, map[string]int64{base + ".1": 22, base: 5}, syncGroup(t, ci, base))
	assert.EqualValues(t, 7, ci.docsIndexed.Load()-before, "only the 7 new lines are read")

	// Compression replaces access.log.1 with access.log.2.gz: same content,
	// nothing new to read.
	content, err := os.ReadFile(base + ".1")
	require.NoError(t, err)
	writeGzip(t, base+".2.gz", string(content))
	require.NoError(t, os.Remove(base+".1"))
	before = ci.docsIndexed.Load()
	assert.Equal(t, map[string]int64{base + ".2.gz": 22, base: 5}, syncGroup(t, ci, base))
	assert.Zero(t, ci.docsIndexed.Load()-before, "the compressed copy is recognized without reading it")

	// logrotate deletes the oldest archive: its rows go with it.
	require.NoError(t, os.Remove(base+".2.gz"))
	assert.Equal(t, map[string]int64{base: 5}, syncGroup(t, ci, base))
}

// copytruncate copies the live log aside and truncates it. The copy is the
// content already indexed; the truncated file starts a new one.
func TestColumnarIndexerHandlesCopyTruncate(t *testing.T) {
	ci, logDir := newTestColumnarIndexer(t)
	base := filepath.Join(logDir, "access.log")

	require.NoError(t, os.WriteFile(base, []byte(accessLogLines(0, 10)), 0o600))
	syncGroup(t, ci, base)

	content, err := os.ReadFile(base)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(base+".1", content, 0o600))
	require.NoError(t, os.Truncate(base, 0))
	assert.Equal(t, map[string]int64{base + ".1": 10}, syncGroup(t, ci, base), "an empty live log has nothing to index yet")

	require.NoError(t, os.WriteFile(base, []byte(accessLogLines(50, 3)), 0o600))
	assert.Equal(t, map[string]int64{base + ".1": 10, base: 3}, syncGroup(t, ci, base))
}

// A line nginx is still writing is left for the next round, so it is never
// stored cut in half.
func TestColumnarIndexerWaitsForTheLastLineToFinish(t *testing.T) {
	ci, logDir := newTestColumnarIndexer(t)
	base := filepath.Join(logDir, "access.log")

	full := accessLogLines(0, 4)
	partial := accessLogLines(4, 1)
	require.NoError(t, os.WriteFile(base, []byte(full+partial[:len(partial)/2]), 0o600))
	assert.Equal(t, map[string]int64{base: 4}, syncGroup(t, ci, base))

	f, err := os.OpenFile(base, os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteString(partial[len(partial)/2:])
	require.NoError(t, err)
	require.NoError(t, f.Close())
	assert.Equal(t, map[string]int64{base: 5}, syncGroup(t, ci, base))

	res, err := ci.Store().Search(context.Background(), &logstore.Query{Text: "/position/4 "}, logstore.SearchOptions{Limit: 1})
	require.NoError(t, err)
	require.EqualValues(t, 1, res.Total)
	assert.Equal(t, strings.TrimSuffix(partial, "\n"), res.Hits[0].Row.Raw)
}

// Moved paths survive a restart: the catalog records where each content
// lives now.
func TestColumnarIndexerRemembersMovedContentAcrossRestart(t *testing.T) {
	ci, logDir := newTestColumnarIndexer(t)
	base := filepath.Join(logDir, "access.log")
	require.NoError(t, os.WriteFile(base, []byte(accessLogLines(0, 6)), 0o600))
	syncGroup(t, ci, base)
	require.NoError(t, os.Rename(base, base+".1"))
	require.NoError(t, os.WriteFile(base, []byte(accessLogLines(10, 2)), 0o600))
	syncGroup(t, ci, base)

	require.NoError(t, ci.Stop())
	require.NoError(t, ci.Start(context.Background()))
	before := ci.docsIndexed.Load()
	assert.Equal(t, map[string]int64{base + ".1": 6, base: 2}, syncGroup(t, ci, base))
	assert.Zero(t, ci.docsIndexed.Load()-before, "nothing is read again after a restart")
}

// Without the city database, visitors are still located by country from the
// database embedded in the binary.
func TestCountryOnlyGeoIPUsesTheEmbeddedDatabase(t *testing.T) {
	loc, err := countryOnlyGeoIP{}.Search("8.8.8.8")
	require.NoError(t, err)
	assert.Equal(t, "US", loc.RegionCode)
	assert.Empty(t, loc.City)

	loc, err = countryOnlyGeoIP{}.Search("192.168.1.1")
	require.NoError(t, err)
	assert.Empty(t, loc.RegionCode, "private addresses have no country")

	_, err = countryOnlyGeoIP{}.Search("-")
	assert.Error(t, err)
}
