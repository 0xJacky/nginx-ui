package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newReadPositionTestServices starts an indexer backed by a temporary index
// directory and a log file manager backed by a temporary metadata database.
func newReadPositionTestServices(t *testing.T, name string) (*ParallelIndexer, *LogFileManager, string) {
	t.Helper()

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
	allowLogDir(t, logDir)

	config := DefaultIndexerConfig()
	config.IndexPath = t.TempDir()
	pi := NewParallelIndexer(config, NewGroupedShardManager(config))
	require.NoError(t, pi.Start(context.Background()))
	t.Cleanup(func() { _ = pi.Stop() })

	lfm := NewLogFileManager()
	lfm.SetIndexer(pi)

	return pi, lfm, logDir
}

// accessLogLines returns count parseable access log lines, numbered from first.
func accessLogLines(first, count int) string {
	var builder strings.Builder
	baseTime := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	for i := first; i < first+count; i++ {
		fmt.Fprintf(&builder, `198.51.100.%d - - [%s] "GET /position/%d HTTP/1.1" 200 %d "-" "read-position-test"`+"\n",
			i%250+1, baseTime.Add(time.Duration(i)*time.Second).Format("02/Jan/2006:15:04:05 -0700"), i, 100+i)
	}
	return builder.String()
}

// TestSaveIndexMetadataRecordsReadPosition is the regression test for issue
// #2054: a full index used to leave LastPosition at 0, so the incremental job
// took the file for inconsistent and re-read it from the start on every run.
func TestSaveIndexMetadataRecordsReadPosition(t *testing.T) {
	pi, lfm, logDir := newReadPositionTestServices(t, "read-position")

	logPath := filepath.Join(logDir, "access.log")
	require.NoError(t, os.WriteFile(logPath, []byte(accessLogLines(0, 5)), 0o600))
	info, err := os.Stat(logPath)
	require.NoError(t, err)

	startTime := time.Now()
	docCount, minTime, maxTime, err := pi.IndexSingleFileWithProgress(logPath, nil)
	require.NoError(t, err)
	require.EqualValues(t, 5, docCount)

	require.NoError(t, lfm.SaveIndexMetadata(logPath, docCount, startTime, time.Since(startTime), minTime, maxTime))

	record, err := lfm.GetPersistence().GetLogIndex(logPath)
	require.NoError(t, err)
	require.Equal(t, info.Size(), record.LastPosition, "a full index must record where it stopped reading")
	require.Equal(t, info.Size(), record.LastSize)
	require.EqualValues(t, 5, record.DocumentCount)

	_, ok := pi.takeReadPosition(logPath)
	require.False(t, ok, "saving the metadata consumes the recorded position")
}

func TestSaveIndexMetadataRecordsReadPositionOfRotatedFile(t *testing.T) {
	pi, lfm, logDir := newReadPositionTestServices(t, "read-position-rotated")

	logPath := filepath.Join(logDir, "access.log")
	require.NoError(t, os.WriteFile(logPath, []byte(accessLogLines(0, 5)), 0o600))
	indexedSize := int64(len(accessLogLines(0, 5)))

	docCount, minTime, maxTime, err := pi.IndexSingleFileWithProgress(logPath, nil)
	require.NoError(t, err)

	// The file is rotated between the read and the metadata save.
	require.NoError(t, os.WriteFile(logPath, []byte(accessLogLines(100, 1)), 0o600))

	require.NoError(t, lfm.SaveIndexMetadata(logPath, docCount, time.Now(), time.Second, minTime, maxTime))

	record, err := lfm.GetPersistence().GetLogIndex(logPath)
	require.NoError(t, err)
	require.Equal(t, indexedSize, record.LastPosition)
	require.Equal(t, indexedSize, record.LastSize,
		"the recorded size must cover what was read, so the next run sees the shrink and re-reads the new file")
}

func TestDeleteIndexByFilesDropsDocumentsAndReadPosition(t *testing.T) {
	pi, _, logDir := newReadPositionTestServices(t, "delete-by-files")

	logPath := filepath.Join(logDir, "access.log")
	otherPath := filepath.Join(logDir, "other.log")
	require.NoError(t, os.WriteFile(logPath, []byte(accessLogLines(0, 4)), 0o600))
	require.NoError(t, os.WriteFile(otherPath, []byte(accessLogLines(10, 2)), 0o600))

	_, _, _, err := pi.IndexSingleFileWithProgress(logPath, nil)
	require.NoError(t, err)
	_, _, _, err = pi.IndexSingleFileWithProgress(otherPath, nil)
	require.NoError(t, err)

	require.NoError(t, pi.DeleteIndexByFiles([]string{logPath}))

	count, err := pi.CountDocsByFilePath(logPath)
	require.NoError(t, err)
	require.Zero(t, count)

	count, err = pi.CountDocsByFilePath(otherPath)
	require.NoError(t, err)
	require.EqualValues(t, 2, count, "documents of other files must survive")

	_, ok := pi.takeReadPosition(logPath)
	require.False(t, ok, "a position must not outlive the documents it described")
	_, ok = pi.takeReadPosition(otherPath)
	require.True(t, ok)
}

func TestDeleteIndexByMainLogPathDropsRotatedFiles(t *testing.T) {
	pi, _, logDir := newReadPositionTestServices(t, "delete-by-main-log-path")

	logPath := filepath.Join(logDir, "access.log")
	rotatedPath := logPath + ".1"
	otherPath := filepath.Join(logDir, "other.log")
	require.NoError(t, os.WriteFile(logPath, []byte(accessLogLines(0, 3)), 0o600))
	require.NoError(t, os.WriteFile(rotatedPath, []byte(accessLogLines(10, 2)), 0o600))
	require.NoError(t, os.WriteFile(otherPath, []byte(accessLogLines(20, 1)), 0o600))

	for _, path := range []string{logPath, rotatedPath, otherPath} {
		_, _, _, err := pi.IndexSingleFileWithProgress(path, nil)
		require.NoError(t, err)
	}

	count, err := pi.CountDocsByMainLogPath(logPath)
	require.NoError(t, err)
	require.EqualValues(t, 5, count)

	require.NoError(t, pi.DeleteIndexByMainLogPath(logPath))

	count, err = pi.CountDocsByMainLogPath(logPath)
	require.NoError(t, err)
	require.Zero(t, count, "the live file and its rotated siblings must all be dropped")

	count, err = pi.CountDocsByFilePath(otherPath)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
}
