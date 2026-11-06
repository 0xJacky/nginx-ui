package indexer

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/require"
)

// indexGroup indexes every given file of a log group and records the group
// metadata the way the indexing jobs do.
func indexGroup(t *testing.T, pi LogIndexer, lfm *LogFileManager, mainPath string, files ...string) {
	t.Helper()
	for _, file := range files {
		docs, minTime, maxTime, err := pi.IndexSingleFileWithProgress(file, nil)
		require.NoError(t, err)
		require.NoError(t, lfm.SaveIndexMetadata(file, docs, time.Now(), time.Millisecond, minTime, maxTime))
	}
	require.Equal(t, mainPath, getMainLogPathFromFile(files[0]))
}

func listedGroups(lfm *LogFileManager) []string {
	var paths []string
	for _, log := range lfm.GetAllLogsWithIndexGrouped() {
		paths = append(paths, log.Path)
	}
	sort.Strings(paths)
	return paths
}

func groupRowCount(t *testing.T, mainPath string) int {
	t.Helper()
	q := query.NginxLogIndex
	rows, err := q.Where(q.MainLogPath.Eq(mainPath)).Find()
	require.NoError(t, err)
	return len(rows)
}

// TestPruneStaleLogGroups is the regression test for issue #2011: the access
// log of a deleted site stayed in the log list forever, even after its file
// had been deleted, because every persisted index row was listed.
func TestPruneStaleLogGroups(t *testing.T) {
	pi, lfm, logDir := newReadPositionTestServices(t, "prune-stale-log-groups")

	write := func(name string, first, count int) string {
		path := filepath.Join(logDir, name)
		require.NoError(t, os.WriteFile(path, []byte(accessLogLines(first, count)), 0o600))
		return path
	}

	// A deleted site: indexed with a rotated sibling, then unconfigured and
	// its files removed.
	deleted := write("deleted.access.log", 0, 3)
	deletedRotated := write("deleted.access.log.1", 10, 2)
	indexGroup(t, pi, lfm, deleted, deleted, deletedRotated)
	require.NoError(t, os.Remove(deleted))
	require.NoError(t, os.Remove(deletedRotated))

	// A configured log with a rotated file: everything must stay.
	live := write("live.access.log", 20, 2)
	liveRotated := write("live.access.log.1", 30, 2)
	lfm.AddLogPath(live, "access", filepath.Base(live), "/etc/nginx/sites-enabled/live.conf")
	indexGroup(t, pi, lfm, live, live, liveRotated)

	// A log no configuration declares any more, but still on disk.
	unconfigured := write("unconfigured.access.log", 40, 1)
	indexGroup(t, pi, lfm, unconfigured, unconfigured)

	// A configured log whose file is temporarily missing (rotated away).
	rotatedAway := write("rotated-away.access.log", 50, 1)
	lfm.AddLogPath(rotatedAway, "access", filepath.Base(rotatedAway), "/etc/nginx/sites-enabled/rotated.conf")
	indexGroup(t, pi, lfm, rotatedAway, rotatedAway)
	require.NoError(t, os.Remove(rotatedAway))

	require.Equal(t, []string{live, rotatedAway, unconfigured}, listedGroups(lfm),
		"a group that is neither configured nor on disk must not be listed")

	stale, err := lfm.StaleLogGroups()
	require.NoError(t, err)
	require.Equal(t, []string{deleted}, stale)

	lfm.PruneStaleLogGroups(nil)

	require.Zero(t, groupRowCount(t, deleted), "the stale group's metadata must be deleted")
	count, err := pi.CountDocsByMainLogPath(deleted)
	require.NoError(t, err)
	require.Zero(t, count, "the stale group's documents must be deleted")

	require.Equal(t, 2, groupRowCount(t, live), "a live group keeps its rotated files")
	count, err = pi.CountDocsByMainLogPath(live)
	require.NoError(t, err)
	require.EqualValues(t, 4, count)
	_, err = os.Stat(liveRotated)
	require.NoError(t, err, "log files are never deleted")

	require.Equal(t, 1, groupRowCount(t, unconfigured))
	require.Equal(t, 1, groupRowCount(t, rotatedAway))
	require.Equal(t, []string{live, rotatedAway, unconfigured}, listedGroups(lfm))
}

func TestPruneStaleLogGroupsSkipsBusyGroups(t *testing.T) {
	_, lfm, logDir := newReadPositionTestServices(t, "prune-stale-busy")

	persistence := lfm.GetPersistence()

	indexing := filepath.Join(logDir, "indexing.access.log")
	require.NoError(t, persistence.SetIndexStatus(indexing, string(IndexStatusIndexing), 0, ""))

	locked := filepath.Join(logDir, "locked.access.log")
	require.NoError(t, persistence.SetIndexStatus(locked, string(IndexStatusIndexed), 0, ""))

	stale, err := lfm.StaleLogGroups()
	require.NoError(t, err)
	require.Equal(t, []string{locked}, stale, "a group with a file being indexed is not stale")

	lfm.PruneStaleLogGroups(func(mainLogPath string) bool { return mainLogPath == locked })

	require.Equal(t, 1, groupRowCount(t, indexing))
	require.Equal(t, 1, groupRowCount(t, locked), "a group held by a task is retried later")

	lfm.PruneStaleLogGroups(nil)
	require.Zero(t, groupRowCount(t, locked))
}
