package nginx_log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	nginxlog "github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestRebuildAllFilesDiscoversLogGroups is the regression test for the
// follow-up on issue #1787.
//
// The reporter turned advanced indexing off and on again and then triggered a
// full rebuild. StopServices dropped the LogFileManager holding every log path
// discovered from the nginx configuration, InitializeServices created a new
// empty one, and the rebuild cleared the index metadata before enumerating the
// log groups. Both sources of log groups were therefore empty, and the rebuild
// logged a successful completion right after "Processing 0 log groups".
//
// The subtests share one metadata database on purpose: the services install
// background goroutines, so swapping the global database handle between
// scenarios would race with them.
func TestRebuildAllFilesDiscoversLogGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping index rebuild integration test in short mode")
	}

	cache.InitInMemoryCache()

	tempDir := t.TempDir()
	logsDir := filepath.Join(tempDir, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	setupRebuildTestEnvironment(t, tempDir, logsDir)

	nginxlog.StopServices()
	t.Cleanup(nginxlog.StopServices)

	// The scenario from issue #1787 itself: the nginx configuration declares no
	// access_log at all - the Homebrew nginx.conf ships with every access_log
	// line commented out - and the log file is only known through
	// nginx.AccessLogPath in app.ini. The log preview worked because it reads
	// that setting directly, but nothing ever registered the path as an
	// indexable log group, so a full rebuild had nothing to process.
	t.Run("group discovered from the nginx default access log path", func(t *testing.T) {
		logPath := writeAccessLog(t, logsDir, "default-access.log", 3)

		originalAccessLogPath := settings.NginxSettings.AccessLogPath
		settings.NginxSettings.AccessLogPath = logPath
		t.Cleanup(func() {
			settings.NginxSettings.AccessLogPath = originalAccessLogPath
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		nginxlog.StopServices()
		require.Empty(t, nginxlog.GetAllLogPaths(),
			"no access_log directive has been scanned, so nothing is registered yet")

		nginxlog.InitializeServices(ctx)

		modernIndexer := nginxlog.GetIndexer()
		require.NotNil(t, modernIndexer)
		logFileManager := nginxlog.GetLogFileManager()
		require.NotNil(t, logFileManager)
		require.NoError(t, logFileManager.DeleteAllIndexMetadata())

		require.Equal(t, []string{logPath}, groupedPaths(nginxlog.GetAllLogsWithIndexGrouped()),
			"the nginx default access log must be listed as a log group")

		rebuildAllFiles(modernIndexer, logFileManager, nil)

		assertIndexedGroup(t, logPath, 3)
		nginxlog.StopServices()
	})

	// A log group known only from the nginx configuration must be rebuilt even
	// after advanced indexing has been switched off and on again.
	t.Run("group discovered from the nginx configuration", func(t *testing.T) {
		logPath := writeAccessLog(t, logsDir, "config-discovered.log", 4)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		nginxlog.StopServices()
		nginxlog.InitializeServices(ctx)
		nginxlog.AddLogPath(logPath, "access", filepath.Base(logPath), filepath.Join(tempDir, "nginx.conf"))
		require.NotEmpty(t, nginxlog.GetAllLogsWithIndexGrouped())

		// The reporter's exact sequence: disable advanced indexing, enable it
		// again, then rebuild without waiting for a new config scan.
		nginxlog.StopServices()
		nginxlog.InitializeServices(ctx)

		modernIndexer := nginxlog.GetIndexer()
		require.NotNil(t, modernIndexer)
		logFileManager := nginxlog.GetLogFileManager()
		require.NotNil(t, logFileManager)

		// No metadata at all, so the log group can only come from the
		// configuration scan. This is the state the rebuild used to create for
		// itself by deleting all metadata before enumerating.
		require.NoError(t, logFileManager.DeleteAllIndexMetadata())

		rebuildAllFiles(modernIndexer, logFileManager, nil)

		assertIndexedGroup(t, logPath, 4)
		nginxlog.StopServices()
	})

	// A log group known only from previous index metadata must survive the
	// metadata wipe performed by the rebuild itself.
	t.Run("group known only from index metadata", func(t *testing.T) {
		logPath := writeAccessLog(t, logsDir, "metadata-only.log", 2)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		nginxlog.StopServices()
		nginxlog.InitializeServices(ctx)

		modernIndexer := nginxlog.GetIndexer()
		require.NotNil(t, modernIndexer)
		logFileManager := nginxlog.GetLogFileManager()
		require.NotNil(t, logFileManager)

		require.NoError(t, logFileManager.DeleteAllIndexMetadata())
		require.NoError(t, logFileManager.SaveIndexMetadata(logPath, 0, time.Now(), 0, nil, nil))

		rebuildAllFiles(modernIndexer, logFileManager, nil)

		assertIndexedGroup(t, logPath, 2)
		nginxlog.StopServices()
	})
}

// groupedPaths returns the paths of a grouped log list in a stable order.
func groupedPaths(logs []*nginxlog.NginxLogWithIndex) []string {
	paths := make([]string, 0, len(logs))
	for _, log := range logs {
		paths = append(paths, log.Path)
	}
	sort.Strings(paths)
	return paths
}

// assertIndexedGroup checks that a full rebuild produced an indexed record with
// the expected document count for the given log group.
func assertIndexedGroup(t *testing.T, logPath string, wantDocuments uint64) {
	t.Helper()

	q := query.NginxLogIndex
	record, err := q.Where(q.Path.Eq(logPath)).First()
	require.NoError(t, err, "the full rebuild must have indexed %s", logPath)
	require.Equal(t, string(indexer.IndexStatusIndexed), record.IndexStatus)
	require.Equal(t, wantDocuments, record.DocumentCount)
}

// writeAccessLog creates an nginx access log with the requested number of
// parseable entries and returns its path.
func writeAccessLog(t *testing.T, logsDir, name string, entries int) string {
	t.Helper()

	var contents strings.Builder
	baseTime := time.Date(2026, time.August, 12, 16, 0, 0, 0, time.UTC)
	for i := 0; i < entries; i++ {
		_, err := fmt.Fprintf(&contents,
			`203.0.113.%d - - [%s] "GET /rebuild/%d HTTP/1.1" 200 %d "-" "rebuild-regression-test"`+"\n",
			i+1,
			baseTime.Add(time.Duration(i)*time.Second).Format("02/Jan/2006:15:04:05 -0700"),
			i,
			100+i,
		)
		require.NoError(t, err)
	}

	logPath := filepath.Join(logsDir, name)
	require.NoError(t, os.WriteFile(logPath, []byte(contents.String()), 0o600))
	return logPath
}

// setupRebuildTestEnvironment points the settings and the metadata database at
// a throwaway directory and restores the previous globals afterwards.
func setupRebuildTestEnvironment(t *testing.T, tempDir, logsDir string) {
	t.Helper()

	originalNginxSettings := *settings.NginxSettings
	originalNginxLogSettings := *settings.NginxLogSettings
	originalModelDB := model.UseDB()
	originalQueryDB := query.Q.UnderlyingDB()

	metadataDB, err := gorm.Open(sqlite.Open(filepath.Join(tempDir, "metadata.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, metadataDB.AutoMigrate(&model.NginxLogIndex{}))
	model.Use(metadataDB)
	query.Use(metadataDB)
	query.SetDefault(metadataDB)

	t.Cleanup(func() {
		*settings.NginxSettings = originalNginxSettings
		*settings.NginxLogSettings = originalNginxLogSettings
		if originalModelDB != nil {
			model.Use(originalModelDB)
		}
		if originalQueryDB != nil {
			query.Use(originalQueryDB)
			query.SetDefault(originalQueryDB)
		}
	})

	settings.NginxSettings.LogDirWhiteList = []string{logsDir}
	settings.NginxLogSettings.IndexingEnabled = true
	settings.NginxLogSettings.IndexPath = filepath.Join(tempDir, "index")

	// Point the default log settings at a directory so the nginx installation of
	// the machine running the test cannot register log paths of its own:
	// nginx.GetAccessLogPath falls back to `nginx -V` when the setting is empty,
	// and utils.IsValidLogPath rejects anything that is not a regular file.
	noDefaultLog := filepath.Join(tempDir, "no-default-log")
	require.NoError(t, os.MkdirAll(noDefaultLog, 0o755))
	settings.NginxSettings.AccessLogPath = noDefaultLog
	settings.NginxSettings.ErrorLogPath = noDefaultLog
}
