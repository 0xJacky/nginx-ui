package nginx_log

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTaskSchedulerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.NginxLogIndex{}))

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	return db
}

func TestNeedsRecovery(t *testing.T) {
	ts := &TaskScheduler{}

	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{
			name:     "indexing in progress should be recovered",
			status:   string(indexer.IndexStatusIndexing),
			expected: true,
		},
		{
			name:     "queued task should be recovered",
			status:   string(indexer.IndexStatusQueued),
			expected: true,
		},
		{
			name:     "not indexed task should be recovered for initial indexing",
			status:   string(indexer.IndexStatusNotIndexed),
			expected: true,
		},
		{
			name:     "indexed task should not be recovered",
			status:   string(indexer.IndexStatusIndexed),
			expected: false,
		},
		{
			name:     "error task should not be recovered when last indexed is zero",
			status:   string(indexer.IndexStatusError),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &NginxLogWithIndex{
				Path:        "/var/log/nginx/access.log",
				IndexStatus: tt.status,
			}
			assert.Equal(t, tt.expected, ts.needsRecovery(log))
		})
	}
}

func TestTaskScheduler_IsTaskInProgress(t *testing.T) {
	ts := NewTaskScheduler(context.Background())
	logPath := "/var/log/nginx/access.log"

	assert.False(t, ts.IsTaskInProgress(logPath))

	lock := ts.acquireTaskLock(logPath)
	lock.Lock()
	defer lock.Unlock()

	assert.True(t, ts.IsTaskInProgress(logPath))
}

func TestTaskScheduler_AcquireTaskLock(t *testing.T) {
	ts := NewTaskScheduler(context.Background())
	logPath := "/var/log/nginx/access.log"

	lock, release := ts.AcquireTaskLock(logPath)
	assert.NotNil(t, lock)

	// A second acquire for the same path should report in progress.
	assert.True(t, ts.IsTaskInProgress(logPath))

	release()
	assert.False(t, ts.IsTaskInProgress(logPath))
}

func TestTaskScheduler_RecoverUnfinishedTasks_NilServices(t *testing.T) {
	ts := NewTaskScheduler(context.Background())
	ts.logFileManager = nil
	ts.modernIndexer = nil

	err := ts.RecoverUnfinishedTasks(context.Background())
	assert.NoError(t, err)
}

func TestTaskScheduler_RecoverUnfinishedTasks_InitialIndexing(t *testing.T) {
	setupTaskSchedulerTestDB(t)

	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "access.log")
	require.NoError(t, os.WriteFile(logPath, []byte(
		`127.0.0.1 - - [22/Jul/2026:00:00:00 +0800] "GET / HTTP/1.1" 200 42 "-" "test"`+"\n"+
			`127.0.0.1 - - [22/Jul/2026:00:00:01 +0800] "GET /api HTTP/1.1" 200 84 "-" "test"`+"\n",
	), 0644))

	// Allow the indexer to access the test log file.
	settings.NginxSettings.LogDirWhiteList = []string{tempDir}
	t.Cleanup(func() {
		settings.NginxSettings.LogDirWhiteList = nil
	})

	// Initialize services state so GetLogFileManager/GetIndexer work.
	servicesMutex.Lock()
	oldServicesInitialized := servicesInitialized
	oldGlobalLogFileManager := globalLogFileManager
	oldGlobalIndexer := globalIndexer
	oldGlobalSearcher := globalSearcher
	t.Cleanup(func() {
		servicesMutex.Lock()
		servicesInitialized = oldServicesInitialized
		globalLogFileManager = oldGlobalLogFileManager
		globalIndexer = oldGlobalIndexer
		globalSearcher = oldGlobalSearcher
		servicesMutex.Unlock()
	})

	indexDir := filepath.Join(tempDir, "index")
	require.NoError(t, os.MkdirAll(indexDir, 0755))

	indexer.InitLogParser()

	indexerConfig := indexer.DefaultIndexerConfig()
	indexerConfig.IndexPath = indexDir
	shardManager := indexer.NewGroupedShardManager(indexerConfig)
	idx := indexer.NewParallelIndexer(indexerConfig, shardManager)
	require.NoError(t, idx.Start(context.Background()))
	t.Cleanup(func() {
		_ = idx.Stop()
	})

	lfm := indexer.NewLogFileManager()
	lfm.SetIndexer(idx)
	lfm.AddLogPath(logPath, "access", "access.log", "/etc/nginx/nginx.conf")

	searchService := searcher.NewSearcher(searcher.DefaultSearcherConfig(), nil)
	t.Cleanup(func() {
		_ = searchService.Stop()
	})

	globalLogFileManager = lfm
	globalIndexer = idx
	globalSearcher = searchService
	servicesInitialized = true
	servicesMutex.Unlock()

	// The initial shard refresh runs before the first index exists. It must
	// remain an idle state instead of executing a search that can only fail.
	servicesMutex.Lock()
	updateSearcherShardsLocked()
	servicesMutex.Unlock()
	require.False(t, searchService.IsHealthy())
	require.Empty(t, searchService.GetShards())
	require.Zero(t, searchService.GetStats().TotalSearches)

	ts := NewTaskScheduler(context.Background())
	err := ts.RecoverUnfinishedTasks(context.Background())
	require.NoError(t, err)

	// Wait for the background indexing task to complete and persist the result.
	q := query.NginxLogIndex
	var record *model.NginxLogIndex
	require.Eventually(t, func() bool {
		rec, err := q.Where(q.Path.Eq(logPath)).First()
		if err != nil {
			return false
		}
		record = rec
		return rec.IndexStatus == string(indexer.IndexStatusIndexed) && rec.DocumentCount > 0
	}, 30*time.Second, 200*time.Millisecond, "indexing task did not complete")

	assert.Equal(t, string(indexer.IndexStatusIndexed), record.IndexStatus)
	assert.Greater(t, record.DocumentCount, uint64(0))
	require.Eventually(t, func() bool {
		return searchService.IsHealthy() && len(searchService.GetShards()) > 0
	}, 10*time.Second, 100*time.Millisecond, "searcher did not receive the initial index shards")
}
