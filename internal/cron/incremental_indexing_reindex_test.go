package cron

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPlanIncrementalRun(t *testing.T) {
	tests := []struct {
		name         string
		existing     *model.NginxLogIndex
		currentSize  int64
		isGzipped    bool
		wantMode     incrementalMode
		wantPosition int64
	}{
		{name: "no metadata", existing: nil, currentSize: 100, wantMode: incrementalReindexFile},
		{name: "gzip", existing: &model.NginxLogIndex{LastSize: 50, LastPosition: 50}, currentSize: 100, isGzipped: true, wantMode: incrementalReindexFile},
		{name: "grown", existing: &model.NginxLogIndex{LastSize: 50, LastPosition: 50}, currentSize: 100, wantMode: incrementalAppend, wantPosition: 50},
		{name: "unchanged", existing: &model.NginxLogIndex{LastSize: 100, LastPosition: 100}, currentSize: 100, wantMode: incrementalSkip},
		{name: "rotated", existing: &model.NginxLogIndex{LastSize: 100, LastPosition: 100}, currentSize: 10, wantMode: incrementalReindexGroup},
		{name: "position past end", existing: &model.NginxLogIndex{LastSize: 10, LastPosition: 100}, currentSize: 50, wantMode: incrementalReindexGroup},
		// The state a full index left behind before issue #2054 was fixed.
		{name: "documents without position", existing: &model.NginxLogIndex{LastSize: 100, DocumentCount: 72000}, currentSize: 120, wantMode: incrementalReindexFile},
		{name: "never indexed", existing: &model.NginxLogIndex{}, currentSize: 120, wantMode: incrementalReindexFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, position := planIncrementalRun(tt.existing, tt.currentSize, tt.isGzipped)
			require.Equal(t, tt.wantMode, mode)
			require.Equal(t, tt.wantPosition, position)
		})
	}
}

// newIncrementalTestServices starts an indexer and a log file manager backed
// by throwaway storage, and returns a whitelisted directory for log files.
func newIncrementalTestServices(t *testing.T, name string) (*indexer.ParallelIndexer, *indexer.LogFileManager, string) {
	t.Helper()

	indexer.InitLogParser()

	originalModelDB := model.UseDB()
	originalQueryDB := query.Q.UnderlyingDB()
	database, err := gorm.Open(sqlite.Open(testdb.DSN(t, name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.NginxLogIndex{}))
	model.Use(database)
	query.SetDefault(database)
	t.Cleanup(func() {
		if originalModelDB != nil {
			model.Use(originalModelDB)
		}
		if originalQueryDB != nil {
			query.SetDefault(originalQueryDB)
		}
	})

	logDir := t.TempDir()
	originalWhiteList := settings.NginxSettings.LogDirWhiteList
	settings.NginxSettings.LogDirWhiteList = append(append([]string{}, originalWhiteList...), logDir)
	t.Cleanup(func() { settings.NginxSettings.LogDirWhiteList = originalWhiteList })

	config := indexer.DefaultIndexerConfig()
	config.IndexPath = t.TempDir()
	pi := indexer.NewParallelIndexer(config, indexer.NewGroupedShardManager(config))
	require.NoError(t, pi.Start(context.Background()))
	t.Cleanup(func() { _ = pi.Stop() })

	lfm := indexer.NewLogFileManager()
	lfm.SetIndexer(pi)

	return pi, lfm, logDir
}

// incrementalLogLines returns count parseable access log lines, numbered from first.
func incrementalLogLines(first, count int) string {
	var builder strings.Builder
	baseTime := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	for i := first; i < first+count; i++ {
		fmt.Fprintf(&builder, `192.0.2.%d - - [%s] "GET /incremental/%d HTTP/1.1" 200 %d "-" "incremental-test"`+"\n",
			i%250+1, baseTime.Add(time.Duration(i)*time.Second).Format("02/Jan/2006:15:04:05 -0700"), i, 100+i)
	}
	return builder.String()
}

// fullIndex indexes a file the way the task scheduler does.
func fullIndex(t *testing.T, pi *indexer.ParallelIndexer, lfm *indexer.LogFileManager, logPath string) {
	t.Helper()
	startTime := time.Now()
	docsCountMap, minTime, maxTime, err := pi.IndexLogGroupWithProgress(logPath, nil)
	require.NoError(t, err)
	var total uint64
	for _, count := range docsCountMap {
		total += count
	}
	require.NoError(t, lfm.SaveIndexMetadata(logPath, total, startTime, time.Since(startTime), minTime, maxTime))
}

func appendToFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func requireDocCount(t *testing.T, pi *indexer.ParallelIndexer, filePath string, want uint64) {
	t.Helper()
	count, err := pi.CountDocsByFilePath(filePath)
	require.NoError(t, err)
	require.Equal(t, want, count, "documents indexed for %s", filePath)
}

func requirePositionAtEnd(t *testing.T, lfm *indexer.LogFileManager, logPath string) {
	t.Helper()
	info, err := os.Stat(logPath)
	require.NoError(t, err)
	record, err := lfm.GetPersistence().GetLogIndex(logPath)
	require.NoError(t, err)
	require.Equal(t, info.Size(), record.LastPosition)
}

func TestIncrementalIndexingAfterFullIndexReadsOnlyAppendedLines(t *testing.T) {
	pi, lfm, logDir := newIncrementalTestServices(t, "incremental-append")

	logPath := filepath.Join(logDir, "access.log")
	require.NoError(t, os.WriteFile(logPath, []byte(incrementalLogLines(0, 6)), 0o600))
	fullIndex(t, pi, lfm, logPath)
	requirePositionAtEnd(t, lfm, logPath)

	appendToFile(t, logPath, incrementalLogLines(6, 3))
	require.NoError(t, performSingleFileIncrementalIndexing(logPath, pi, lfm))

	requireDocCount(t, pi, logPath, 9)
	requirePositionAtEnd(t, lfm, logPath)

	// A further run without new data must leave the index alone.
	require.NoError(t, performSingleFileIncrementalIndexing(logPath, pi, lfm))
	requireDocCount(t, pi, logPath, 9)
}

// TestIncrementalIndexingRepairsMissingPositionWithoutDuplicates reproduces
// issue #2054: metadata written by a full index before the fix has documents
// but LastPosition 0. The incremental run re-read the whole file into memory
// and indexed every line a second time under a different ID scheme.
func TestIncrementalIndexingRepairsMissingPositionWithoutDuplicates(t *testing.T) {
	pi, lfm, logDir := newIncrementalTestServices(t, "incremental-missing-position")

	logPath := filepath.Join(logDir, "access.log")
	require.NoError(t, os.WriteFile(logPath, []byte(incrementalLogLines(0, 8)), 0o600))
	fullIndex(t, pi, lfm, logPath)

	// Rewind the metadata to the pre-fix state.
	q := query.NginxLogIndex
	_, err := q.Where(q.Path.Eq(logPath)).UpdateSimple(q.LastPosition.Value(0))
	require.NoError(t, err)

	appendToFile(t, logPath, incrementalLogLines(8, 2))
	require.NoError(t, performSingleFileIncrementalIndexing(logPath, pi, lfm))

	requireDocCount(t, pi, logPath, 10)
	requirePositionAtEnd(t, lfm, logPath)

	record, err := lfm.GetPersistence().GetLogIndex(logPath)
	require.NoError(t, err)
	require.EqualValues(t, 10, record.DocumentCount)
}

func TestIncrementalIndexingReindexesGroupAfterRotation(t *testing.T) {
	pi, lfm, logDir := newIncrementalTestServices(t, "incremental-rotation")

	logPath := filepath.Join(logDir, "access.log")
	require.NoError(t, os.WriteFile(logPath, []byte(incrementalLogLines(0, 7)), 0o600))
	fullIndex(t, pi, lfm, logPath)

	// Rotate: the indexed content moves to access.log.1 and a shorter live
	// file starts over.
	require.NoError(t, os.Rename(logPath, logPath+".1"))
	require.NoError(t, os.WriteFile(logPath, []byte(incrementalLogLines(100, 2)), 0o600))

	require.NoError(t, performSingleFileIncrementalIndexing(logPath, pi, lfm))

	requireDocCount(t, pi, logPath, 2)
	requireDocCount(t, pi, logPath+".1", 7)

	count, err := pi.CountDocsByMainLogPath(logPath)
	require.NoError(t, err)
	require.EqualValues(t, 9, count, "every line of the group must be indexed exactly once")
	requirePositionAtEnd(t, lfm, logPath)
}
