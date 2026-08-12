package nginx_log

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var registryTestDBOnce sync.Once

// useRegistryTestDB installs a shared in-memory metadata database. It is kept
// in memory on purpose: the database handle stays registered globally after the
// test returns, and a file-backed database inside t.TempDir() would be deleted
// underneath the tests that run afterwards.
func useRegistryTestDB(t *testing.T) {
	t.Helper()

	registryTestDBOnce.Do(func() {
		db, err := gorm.Open(sqlite.Open("file:nginx_log_registry_test?mode=memory&cache=shared"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&model.NginxLogIndex{}))

		sqlDB, err := db.DB()
		require.NoError(t, err)
		// Keep at least one connection open so the shared in-memory database is
		// not dropped between tests.
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetConnMaxIdleTime(0)

		model.Use(db)
		query.Use(db)
		query.SetDefault(db)
	})
}

// resetConfigLogRegistry isolates a test from log paths registered by other
// tests and restores the previous content afterwards.
func resetConfigLogRegistry(t *testing.T) {
	t.Helper()

	configLogRegistryMutex.Lock()
	previous := configLogRegistry
	configLogRegistry = make(map[string]*NginxLogCache)
	configLogRegistryMutex.Unlock()

	t.Cleanup(func() {
		configLogRegistryMutex.Lock()
		configLogRegistry = previous
		configLogRegistryMutex.Unlock()
	})
}

// logFileManagerSwapper returns a setter that publishes a LogFileManager the
// way InitializeServices does, and nil the way StopServices does. The original
// service state is restored when the test ends.
func logFileManagerSwapper(t *testing.T) func(*indexer.LogFileManager) {
	t.Helper()

	servicesMutex.Lock()
	originalInitialized := servicesInitialized
	originalManager := globalLogFileManager
	servicesMutex.Unlock()

	t.Cleanup(func() {
		servicesMutex.Lock()
		servicesInitialized = originalInitialized
		globalLogFileManager = originalManager
		servicesMutex.Unlock()
	})

	return func(manager *indexer.LogFileManager) {
		servicesMutex.Lock()
		globalLogFileManager = manager
		servicesInitialized = manager != nil
		servicesMutex.Unlock()
	}
}

func groupedLogPaths(logs []*NginxLogWithIndex) []string {
	paths := make([]string, 0, len(logs))
	for _, log := range logs {
		paths = append(paths, log.Path)
	}
	sort.Strings(paths)
	return paths
}

func cachedLogPaths(logs []*NginxLogCache) []string {
	paths := make([]string, 0, len(logs))
	for _, log := range logs {
		paths = append(paths, log.Path)
	}
	sort.Strings(paths)
	return paths
}

// newDiscoverableAccessLog creates a whitelisted access log file so
// utils.IsValidLogPath accepts the directive path.
func newDiscoverableAccessLog(t *testing.T) string {
	t.Helper()

	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "access.log")
	require.NoError(t, os.WriteFile(logPath, []byte("\n"), 0o600))

	originalWhiteList := settings.NginxSettings.LogDirWhiteList
	settings.NginxSettings.LogDirWhiteList = []string{logDir}
	t.Cleanup(func() {
		settings.NginxSettings.LogDirWhiteList = originalWhiteList
	})

	return logPath
}

// TestScanForLogDirectivesSurvivesLogFileManagerRecreation reproduces the
// regression reported in issue #1787: turning advanced indexing off destroys
// the LogFileManager that owns the discovered log paths, and turning it back on
// builds a new, empty one. Nothing repopulated it until the next periodic
// config scan five minutes later, so a rebuild started right afterwards found
// zero log groups. The paths discovered from the nginx configuration must
// outlive the services and be handed to every new manager.
func TestScanForLogDirectivesSurvivesLogFileManagerRecreation(t *testing.T) {
	cache.InitInMemoryCache()
	useRegistryTestDB(t)
	resetConfigLogRegistry(t)

	logPath := newDiscoverableAccessLog(t)
	configPath := "/etc/nginx/conf.d/regression.conf"
	configContent := []byte("server {\n    access_log " + logPath + " main;\n}\n")

	setManager := logFileManagerSwapper(t)

	// Advanced indexing is on and the config scanner discovers the directive.
	first := indexer.NewLogFileManager()
	setManager(first)
	require.NoError(t, scanForLogDirectives(configPath, configContent))
	require.Equal(t, []string{logPath}, groupedLogPaths(GetAllLogsWithIndexGrouped()))

	// Advanced indexing turned off: StopServices drops the manager.
	setManager(nil)
	require.Equal(t, []string{logPath}, groupedLogPaths(GetAllLogsWithIndexGrouped()),
		"log paths discovered from the nginx configuration must remain listable while indexing is disabled")

	// Advanced indexing turned back on: InitializeServices builds a new manager
	// that starts out empty and has to be seeded from the registry.
	second := indexer.NewLogFileManager()
	setManager(second)
	require.Empty(t, second.GetAllLogPaths(), "a freshly created manager starts without any log path")

	require.Equal(t, 1, SyncDiscoveredLogPaths())
	require.Equal(t, []string{logPath}, cachedLogPaths(second.GetAllLogPaths()))
	require.Equal(t, []string{logPath}, groupedLogPaths(GetAllLogsWithIndexGrouped()))
}

// TestScanForLogDirectivesBeforeServicesStart covers the boot ordering where
// the config scan runs before the indexing services finish starting. The paths
// must not be lost: the manager created later is seeded from the registry.
func TestScanForLogDirectivesBeforeServicesStart(t *testing.T) {
	cache.InitInMemoryCache()
	useRegistryTestDB(t)
	resetConfigLogRegistry(t)

	logPath := newDiscoverableAccessLog(t)
	configPath := "/etc/nginx/conf.d/early.conf"

	setManager := logFileManagerSwapper(t)
	setManager(nil)

	require.NoError(t, scanForLogDirectives(configPath,
		[]byte("server {\n    access_log "+logPath+";\n}\n")))
	require.Equal(t, []string{logPath}, cachedLogPaths(GetAllLogPaths()))

	manager := indexer.NewLogFileManager()
	setManager(manager)
	require.Equal(t, 1, seedLogFileManager(manager))
	require.Equal(t, []string{logPath}, cachedLogPaths(manager.GetAllLogPaths()))
}

// TestRemoveLogPathsFromConfigClearsRegistry makes sure the registry does not
// keep serving log paths whose directive was removed from the configuration.
func TestRemoveLogPathsFromConfigClearsRegistry(t *testing.T) {
	cache.InitInMemoryCache()
	useRegistryTestDB(t)
	resetConfigLogRegistry(t)

	logPath := newDiscoverableAccessLog(t)
	configPath := "/etc/nginx/conf.d/removable.conf"

	setManager := logFileManagerSwapper(t)
	manager := indexer.NewLogFileManager()
	setManager(manager)

	require.NoError(t, scanForLogDirectives(configPath,
		[]byte("server {\n    access_log "+logPath+";\n}\n")))
	require.Equal(t, []string{logPath}, cachedLogPaths(GetAllLogPaths()))

	// The directive is gone from the config file: the scanner replays the file
	// with content that no longer declares it.
	require.NoError(t, scanForLogDirectives(configPath, []byte("server {\n}\n")))
	require.Empty(t, GetAllLogPaths())

	// The removal must survive a manager restart as well.
	setManager(indexer.NewLogFileManager())
	require.Zero(t, SyncDiscoveredLogPaths())
	require.Empty(t, GetAllLogsWithIndexGrouped())
}
