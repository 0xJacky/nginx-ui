package cron

import (
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var testAccessLogLines = []string{
	`127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /a HTTP/1.1" 200 10 "-" "agent"`,
	`127.0.0.2 - - [25/Dec/2023:10:01:00 +0000] "GET /b HTTP/1.1" 200 20 "-" "agent"`,
	`127.0.0.3 - - [25/Dec/2023:10:02:00 +0000] "GET /c HTTP/1.1" 404 30 "-" "agent"`,
	`127.0.0.4 - - [25/Dec/2023:10:03:00 +0000] "POST /d HTTP/1.1" 201 40 "-" "agent"`,
	`127.0.0.5 - - [25/Dec/2023:10:04:00 +0000] "GET /e HTTP/1.1" 500 50 "-" "agent"`,
}

func logContent(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

func writeLog(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func appendLog(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, errors.Join(err, f.Close()))
}

func writeGzipLog(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	gz := gzip.NewWriter(f)
	_, err = gz.Write([]byte(content))
	require.NoError(t, errors.Join(err, gz.Close(), f.Close()))
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Size()
}

// newIndexingEnv starts a real single-shard indexer in a temporary directory,
// a log file manager that persists index metadata into an in-memory DB, and a
// log directory that is on the log path whitelist.
func newIndexingEnv(t *testing.T) (*indexer.ColumnarIndexer, *indexer.LogFileManager, *gorm.DB, string) {
	t.Helper()

	db := useCronTestDB(t, &model.NginxLogIndex{})
	indexer.InitLogParser()

	logDir := t.TempDir()
	originalWhiteList := settings.NginxSettings.LogDirWhiteList
	settings.NginxSettings.LogDirWhiteList = append(append([]string{}, originalWhiteList...), logDir)
	t.Cleanup(func() { settings.NginxSettings.LogDirWhiteList = originalWhiteList })

	config := indexer.DefaultIndexerConfig()
	config.IndexPath = t.TempDir()
	config.ShardCount = 1
	config.WorkerCount = 1
	config.MaxQueueSize = 8
	config.OptimizeInterval = 0
	config.EnableMetrics = false

	pi := indexer.NewColumnarIndexer(config)
	require.NoError(t, pi.Start(context.Background()))
	t.Cleanup(func() { _ = pi.Stop() })

	lfm := indexer.NewLogFileManager()
	lfm.SetIndexer(pi)

	return pi, lfm, db, logDir
}

func storedLogIndex(t *testing.T, db *gorm.DB, path string) model.NginxLogIndex {
	t.Helper()
	var logIndex model.NginxLogIndex
	require.NoError(t, db.Where("path = ?", path).First(&logIndex).Error)
	return logIndex
}

func indexedDocs(t *testing.T, pi indexer.LogIndexer, path string) uint64 {
	t.Helper()
	count, err := pi.CountDocsByFilePath(path)
	require.NoError(t, err)
	return count
}

func TestNeedsIncrementalIndexingDecisions(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "access.log")
	writeLog(t, logPath, logContent(testAccessLogLines[:2]...))
	info, err := os.Stat(logPath)
	require.NoError(t, err)

	// A sparse file is enough to cross the large file threshold without
	// writing 100MB.
	largePath := filepath.Join(dir, "large.log")
	writeLog(t, largePath, "")
	require.NoError(t, os.Truncate(largePath, 101*1024*1024))

	// Stat fails with ENOTDIR, which is not "does not exist".
	notDirPath := filepath.Join(logPath, "child.log")

	older := info.ModTime().Add(-time.Hour)
	lookupFailed := stubLogIndexProvider{err: errors.New("lookup failed")}
	needsIndexing := stubLogIndexProvider{idx: &model.NginxLogIndex{LastIndexed: time.Now().Add(-time.Hour)}}
	upToDate := stubLogIndexProvider{idx: &model.NginxLogIndex{
		LastIndexed:  time.Now(),
		LastModified: info.ModTime(),
		LastSize:     info.Size(),
	}}

	tests := []struct {
		name        string
		log         nginx_log.NginxLogWithIndex
		persistence logIndexProvider
		want        bool
	}{
		{
			name: "already indexing",
			log:  nginx_log.NginxLogWithIndex{Path: logPath, IndexStatus: string(indexer.IndexStatusIndexing)},
			want: false,
		},
		{
			name: "already queued",
			log:  nginx_log.NginxLogWithIndex{Path: logPath, IndexStatus: string(indexer.IndexStatusQueued)},
			want: false,
		},
		{
			name: "file removed keeps historical index",
			log:  nginx_log.NginxLogWithIndex{Path: filepath.Join(dir, "missing.log")},
			want: false,
		},
		{
			name: "stat error",
			log:  nginx_log.NginxLogWithIndex{Path: notDirPath},
			want: false,
		},
		{
			name: "never indexed without persisted metadata",
			log:  nginx_log.NginxLogWithIndex{Path: logPath},
			want: true,
		},
		{
			name:        "metadata lookup failure falls back to never indexed",
			log:         nginx_log.NginxLogWithIndex{Path: logPath},
			persistence: lookupFailed,
			want:        true,
		},
		{
			name: "fallback detects growth",
			log: nginx_log.NginxLogWithIndex{
				Path:         logPath,
				LastIndexed:  time.Now().Unix(),
				LastModified: older.Unix(),
				LastSize:     info.Size() - 1,
			},
			want: true,
		},
		{
			name: "fallback ignores newer mod time without growth",
			log: nginx_log.NginxLogWithIndex{
				Path:         logPath,
				LastIndexed:  time.Now().Unix(),
				LastModified: older.Unix(),
				LastSize:     info.Size(),
			},
			want: false,
		},
		{
			name: "fallback treats unknown size as current size",
			log: nginx_log.NginxLogWithIndex{
				Path:         logPath,
				LastIndexed:  time.Now().Unix(),
				LastModified: older.Unix(),
			},
			want: false,
		},
		{
			name:        "persisted metadata says up to date",
			log:         nginx_log.NginxLogWithIndex{Path: logPath, LastIndexed: time.Now().Unix()},
			persistence: upToDate,
			want:        false,
		},
		{
			name:        "persisted metadata wins over aggregated fallback",
			log:         nginx_log.NginxLogWithIndex{Path: logPath},
			persistence: upToDate,
			want:        false,
		},
		{
			name:        "large file recently indexed is skipped",
			log:         nginx_log.NginxLogWithIndex{Path: largePath, LastIndexed: time.Now().Add(-time.Minute).Unix()},
			persistence: needsIndexing,
			want:        false,
		},
		{
			name:        "large file indexed long ago is checked",
			log:         nginx_log.NginxLogWithIndex{Path: largePath, LastIndexed: time.Now().Add(-time.Hour).Unix()},
			persistence: needsIndexing,
			want:        true,
		},
		{
			name:        "large file never indexed is checked",
			log:         nginx_log.NginxLogWithIndex{Path: largePath},
			persistence: needsIndexing,
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := tt.log
			assert.Equal(t, tt.want, needsIncrementalIndexing(&log, tt.persistence))
		})
	}
}

func TestSetFileIndexStatusRejectsUnusableManager(t *testing.T) {
	tests := []struct {
		name    string
		manager interface{}
	}{
		{name: "nil manager", manager: nil},
		{name: "wrong type", manager: "not a manager"},
		{name: "indexer instead of manager", manager: &indexer.ParallelIndexer{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, setFileIndexStatus("/var/log/nginx/access.log", string(indexer.IndexStatusIndexed), tt.manager))
		})
	}
}

func TestSetFileIndexStatusPersistsStatus(t *testing.T) {
	db := useCronTestDB(t, &model.NginxLogIndex{})
	manager := indexer.NewLogFileManager()
	logPath := filepath.Join(t.TempDir(), "access.log")

	require.NoError(t, setFileIndexStatus(logPath, string(indexer.IndexStatusQueued), manager))
	stored := storedLogIndex(t, db, logPath)
	assert.Equal(t, string(indexer.IndexStatusQueued), stored.IndexStatus)
	assert.GreaterOrEqual(t, stored.QueuePosition, 0)
	assert.Less(t, stored.QueuePosition, 1000)

	require.NoError(t, setFileIndexStatus(logPath, string(indexer.IndexStatusIndexing), manager))
	stored = storedLogIndex(t, db, logPath)
	assert.Equal(t, string(indexer.IndexStatusIndexing), stored.IndexStatus)
	assert.NotNil(t, stored.IndexStartTime)

	require.NoError(t, setFileIndexStatus(logPath, string(indexer.IndexStatusError), manager))
	stored = storedLogIndex(t, db, logPath)
	assert.Equal(t, string(indexer.IndexStatusError), stored.IndexStatus)
	assert.Equal(t, 1, stored.RetryCount)
	assert.NotNil(t, stored.ErrorTime)

	var count int64
	require.NoError(t, db.Model(&model.NginxLogIndex{}).Count(&count).Error)
	assert.EqualValues(t, 1, count, "status updates must reuse the record of the file")
}

func TestPerformSingleFileIncrementalIndexingRejectsBadInput(t *testing.T) {
	useCronTestDB(t, &model.NginxLogIndex{})

	t.Run("missing file", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "access.log")
		require.Error(t, performSingleFileIncrementalIndexing(missing, nil, indexer.NewLogFileManager()))
	})
}

func TestPerformSingleFileIncrementalIndexingReadsOnlyNewContent(t *testing.T) {
	pi, manager, db, logDir := newIndexingEnv(t)
	logPath := filepath.Join(logDir, "access.log")

	// First run indexes the whole file.
	writeLog(t, logPath, logContent(testAccessLogLines[:3]...))
	require.NoError(t, performSingleFileIncrementalIndexing(logPath, pi, manager))

	stored := storedLogIndex(t, db, logPath)
	assert.EqualValues(t, 3, stored.DocumentCount)
	assert.Equal(t, fileSize(t, logPath), stored.LastPosition)
	assert.Equal(t, fileSize(t, logPath), stored.LastSize)
	require.NotNil(t, stored.TimeRangeStart)
	require.NotNil(t, stored.TimeRangeEnd)
	assert.Equal(t, time.Date(2023, 12, 25, 10, 0, 0, 0, time.UTC), stored.TimeRangeStart.UTC())
	assert.Equal(t, time.Date(2023, 12, 25, 10, 2, 0, 0, time.UTC), stored.TimeRangeEnd.UTC())
	assert.EqualValues(t, 3, indexedDocs(t, pi, logPath))

	// Appended lines are indexed on top of the existing documents.
	appendLog(t, logPath, logContent(testAccessLogLines[3:]...))
	require.NoError(t, performSingleFileIncrementalIndexing(logPath, pi, manager))

	stored = storedLogIndex(t, db, logPath)
	assert.EqualValues(t, 5, stored.DocumentCount)
	assert.Equal(t, fileSize(t, logPath), stored.LastPosition)
	assert.Equal(t, time.Date(2023, 12, 25, 10, 0, 0, 0, time.UTC), stored.TimeRangeStart.UTC(),
		"incremental runs must keep the historical start of the range")
	assert.Equal(t, time.Date(2023, 12, 25, 10, 4, 0, 0, time.UTC), stored.TimeRangeEnd.UTC())
	assert.EqualValues(t, 5, indexedDocs(t, pi, logPath), "appended lines must not duplicate earlier documents")

	// An unchanged file is skipped.
	require.NoError(t, performSingleFileIncrementalIndexing(logPath, pi, manager))
	stored = storedLogIndex(t, db, logPath)
	assert.EqualValues(t, 5, stored.DocumentCount)
	assert.EqualValues(t, 5, indexedDocs(t, pi, logPath))
}

func TestPerformSingleFileIncrementalIndexingRestartsAfterRotation(t *testing.T) {
	pi, manager, db, logDir := newIndexingEnv(t)
	logPath := filepath.Join(logDir, "access.log")

	writeLog(t, logPath, logContent(testAccessLogLines...))
	require.NoError(t, performSingleFileIncrementalIndexing(logPath, pi, manager))
	require.EqualValues(t, 5, storedLogIndex(t, db, logPath).DocumentCount)

	// The file shrank: it was rotated and restarted with new content.
	writeLog(t, logPath, logContent(testAccessLogLines[0]))
	require.NoError(t, performSingleFileIncrementalIndexing(logPath, pi, manager))

	stored := storedLogIndex(t, db, logPath)
	assert.EqualValues(t, 1, stored.DocumentCount, "rotation must reset the document count")
	assert.Equal(t, fileSize(t, logPath), stored.LastPosition)
	assert.Equal(t, fileSize(t, logPath), stored.LastSize)
	assert.EqualValues(t, 1, indexedDocs(t, pi, logPath), "documents of the rotated-away content must be removed")
}

func TestPerformSingleFileIncrementalIndexingDropsAppendedSegmentsAfterRotation(t *testing.T) {
	pi, manager, db, logDir := newIndexingEnv(t)
	logPath := filepath.Join(logDir, "access.log")

	// Two runs leave documents under two segment offsets (0 and the append offset).
	writeLog(t, logPath, logContent(testAccessLogLines[:3]...))
	require.NoError(t, performSingleFileIncrementalIndexing(logPath, pi, manager))
	appendLog(t, logPath, logContent(testAccessLogLines[3:]...))
	require.NoError(t, performSingleFileIncrementalIndexing(logPath, pi, manager))
	require.EqualValues(t, 5, indexedDocs(t, pi, logPath))

	writeLog(t, logPath, logContent(testAccessLogLines[0]))
	require.NoError(t, performSingleFileIncrementalIndexing(logPath, pi, manager))

	assert.EqualValues(t, 1, storedLogIndex(t, db, logPath).DocumentCount)
	assert.EqualValues(t, 1, indexedDocs(t, pi, logPath),
		"documents from earlier segments must not survive a rotation")
}

func TestPerformSingleFileIncrementalIndexingResetsInconsistentCount(t *testing.T) {
	pi, manager, db, logDir := newIndexingEnv(t)
	logPath := filepath.Join(logDir, "access.log")
	writeLog(t, logPath, logContent(testAccessLogLines[:2]...))

	// Documents are recorded without a read position, so the file has to be
	// read from the start and the old count must not be added on top.
	require.NoError(t, db.Create(&model.NginxLogIndex{
		Path:          logPath,
		MainLogPath:   logPath,
		DocumentCount: 40,
		LastIndexed:   time.Now().Add(-time.Hour),
	}).Error)

	require.NoError(t, performSingleFileIncrementalIndexing(logPath, pi, manager))

	stored := storedLogIndex(t, db, logPath)
	assert.EqualValues(t, 2, stored.DocumentCount)
	assert.Equal(t, fileSize(t, logPath), stored.LastPosition)
}

func TestPerformSingleFileIncrementalIndexingReindexesGzipFully(t *testing.T) {
	pi, manager, db, logDir := newIndexingEnv(t)
	logPath := filepath.Join(logDir, "access.log.1.gz")
	writeGzipLog(t, logPath, logContent(testAccessLogLines[:3]...))

	// Compressed offsets cannot be resumed, so every run re-reads the file
	// and replaces, rather than adds to, the previous documents.
	for run := 1; run <= 2; run++ {
		require.NoError(t, performSingleFileIncrementalIndexing(logPath, pi, manager), "run %d", run)

		stored := storedLogIndex(t, db, logPath)
		assert.EqualValues(t, 3, stored.DocumentCount, "run %d", run)
		assert.Equal(t, fileSize(t, logPath), stored.LastSize, "run %d", run)
		assert.EqualValues(t, 3, indexedDocs(t, pi, logPath), "run %d", run)
	}
}

func TestIndexFileFromPosition(t *testing.T) {
	// Gzip files never resume at an offset; they are re-indexed as a whole
	// through the full index path, covered by the gzip test above.
	pi, _, _, dir := newIndexingEnv(t)

	t.Run("plain file resumes at offset", func(t *testing.T) {
		logPath := filepath.Join(dir, "plain.log")
		first := logContent(testAccessLogLines[0])
		writeLog(t, logPath, first+logContent(testAccessLogLines[1:3]...))

		count, minTime, maxTime, position, err := indexFileFromPosition(pi, logPath, int64(len(first)))
		require.NoError(t, err)
		assert.EqualValues(t, 2, count)
		assert.Equal(t, fileSize(t, logPath), position)
		require.NotNil(t, minTime)
		require.NotNil(t, maxTime)
		assert.Equal(t, time.Date(2023, 12, 25, 10, 1, 0, 0, time.UTC), minTime.UTC())
		assert.Equal(t, time.Date(2023, 12, 25, 10, 2, 0, 0, time.UTC), maxTime.UTC())
	})

	t.Run("nothing new to read", func(t *testing.T) {
		logPath := filepath.Join(dir, "unchanged.log")
		content := logContent(testAccessLogLines[0])
		writeLog(t, logPath, content)

		count, minTime, maxTime, position, err := indexFileFromPosition(pi, logPath, int64(len(content)))
		require.NoError(t, err)
		assert.Zero(t, count)
		assert.Nil(t, minTime)
		assert.Nil(t, maxTime)
		assert.Equal(t, int64(len(content)), position)
	})

	t.Run("missing file", func(t *testing.T) {
		_, _, _, _, err := indexFileFromPosition(pi, filepath.Join(dir, "missing.log"), 0)
		require.Error(t, err)
	})

}

func TestPerformIncrementalIndexingSkipsWithoutPrerequisites(t *testing.T) {
	db := useCronTestDB(t, &model.NginxLogIndex{})

	original := settings.NginxLogSettings.IndexingEnabled
	t.Cleanup(func() { settings.NginxLogSettings.IndexingEnabled = original })

	// Disabled indexing returns before touching anything; enabled indexing
	// stops because the log services were never initialised in this process.
	for _, enabled := range []bool{false, true} {
		settings.NginxLogSettings.IndexingEnabled = enabled
		performIncrementalIndexing()
	}

	var count int64
	require.NoError(t, db.Model(&model.NginxLogIndex{}).Count(&count).Error)
	assert.Zero(t, count)
}
