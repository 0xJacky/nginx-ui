package plugin

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/ini.v1"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const legacyTestSchema = `CREATE TABLE nginx_log_indices (
	id text PRIMARY KEY,
	created_at datetime,
	updated_at datetime,
	path text NOT NULL,
	main_log_path text,
	last_modified datetime,
	last_size integer DEFAULT 0,
	last_position integer DEFAULT 0,
	last_indexed datetime,
	index_start_time datetime,
	index_duration integer,
	time_range_start datetime,
	time_range_end datetime,
	document_count integer DEFAULT 0,
	enabled numeric DEFAULT true,
	index_status text DEFAULT 'not_indexed',
	error_message text,
	error_time datetime,
	retry_count integer DEFAULT 0,
	queue_position integer DEFAULT 0
)`

func newLegacyTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t, "legacy")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxIdleTime(0)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func createLegacyTable(t *testing.T, db *gorm.DB, rows int) {
	t.Helper()

	require.NoError(t, db.Exec(legacyTestSchema).Error)
	for i := 0; i < rows; i++ {
		require.NoError(t, db.Exec(`INSERT INTO nginx_log_indices
			(id, created_at, updated_at, path, main_log_path, last_modified, last_size, last_position,
			 last_indexed, time_range_start, time_range_end, document_count, enabled, index_status)
			VALUES (?, '2026-01-02 03:04:05+00:00', '2026-01-02 03:04:05+00:00', ?, ?, '2026-01-02 03:04:05+00:00',
			 1000, 900, '2026-01-02 03:04:05+00:00', '2026-01-01 00:00:00+00:00', '2026-01-02 00:00:00+00:00',
			 42, true, 'indexed')`,
			"00000000-0000-4000-8000-00000000000"+string(rune('1'+i)),
			"/var/log/nginx/access"+string(rune('1'+i))+".log",
			"/var/log/nginx/access"+string(rune('1'+i))+".log").Error)
	}
}

type legacyTestEnv struct {
	deps     legacyDeps
	dataDir  string
	confDir  string
	disabled int
	saved    []map[string]any
}

func newLegacyTestEnv(t *testing.T, enabled bool) *legacyTestEnv {
	t.Helper()

	confDir := t.TempDir()
	env := &legacyTestEnv{dataDir: filepath.Join(t.TempDir(), "data"), confDir: confDir}
	env.deps = legacyDeps{
		db:              newLegacyTestDB(t),
		confDir:         confDir,
		env:             func(string) (string, bool) { return "", false },
		indexingEnabled: enabled,
		disableIndexing: func() error { env.disabled++; return nil },
	}
	return env
}

func (e *legacyTestEnv) prepare(stored map[string]any) error {
	return prepareLegacyLogAnalytics(e.deps, e.dataDir, stored, func(values map[string]any) error {
		e.saved = append(e.saved, values)
		return nil
	})
}

func (e *legacyTestEnv) importPath(name string) string {
	return filepath.Join(e.dataDir, legacyImportDir, name)
}

func TestLegacyHandoffWritesSettingsFilesAndSwitchesIndexingOff(t *testing.T) {
	env := newLegacyTestEnv(t, true)
	createLegacyTable(t, env.deps.db, 2)

	conf, err := ini.Load([]byte("[nginx_log]\nIndexingEnabled = true\nIncrementalIndexInterval = 30\n" +
		"MaxConcurrentIndexTasks = 3\nIndexCustomMMDB = corp.mmdb\nGeoMapPath = maps\n"))
	require.NoError(t, err)
	env.deps.conf = conf

	indexDir := filepath.Join(env.confDir, "log-index")
	require.NoError(t, os.MkdirAll(indexDir, 0o755))
	cityDB := filepath.Join(env.confDir, "GeoLite2-City.mmdb")
	require.NoError(t, os.WriteFile(cityDB, []byte("mmdb"), 0o600))

	require.NoError(t, env.prepare(nil))

	require.Len(t, env.saved, 1)
	assert.Equal(t, map[string]any{
		"incremental_index_interval": float64(30),
		"max_concurrent_index_tasks": float64(3),
		"index_custom_mmdb":          filepath.Join(env.confDir, "corp.mmdb"),
		"geo_map_path":               filepath.Join(env.confDir, "maps"),
	}, env.saved[0])

	var rows []map[string]any
	raw, err := os.ReadFile(env.importPath(legacyIndicesFile))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &rows))
	require.Len(t, rows, 2)
	assert.Equal(t, "/var/log/nginx/access1.log", rows[0]["path"])
	assert.Equal(t, "/var/log/nginx/access1.log", rows[0]["main_log_path"])
	assert.Equal(t, float64(42), rows[0]["document_count"])
	assert.Equal(t, "indexed", rows[0]["index_status"])
	assert.Equal(t, "2026-01-01T00:00:00Z", rows[0]["timerange_start"])
	assert.Equal(t, "2026-01-02T00:00:00Z", rows[0]["timerange_end"])
	assert.Equal(t, "00000000-0000-4000-8000-000000000001", rows[0]["id"])

	var info legacyInfo
	raw, err = os.ReadFile(env.importPath(legacyInfoFile))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &info))
	assert.Equal(t, legacyInfo{IndexPath: indexDir, GeoLitePath: cityDB}, info)

	assert.Equal(t, 1, env.disabled)
	assert.FileExists(t, filepath.Join(env.dataDir, legacyDoneMarker))

	// Nothing is left behind next to the import directory.
	entries, err := os.ReadDir(env.dataDir)
	require.NoError(t, err)
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	assert.ElementsMatch(t, []string{legacyImportDir, legacyDoneMarker}, names)
}

func TestLegacyHandoffUsesTheConfiguredIndexPath(t *testing.T) {
	env := newLegacyTestEnv(t, true)
	custom := filepath.Join(t.TempDir(), "my-index")
	require.NoError(t, os.MkdirAll(custom, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(env.confDir, "log-index"), 0o755))
	env.deps.indexPath = custom

	require.NoError(t, env.prepare(nil))

	var info legacyInfo
	raw, err := os.ReadFile(env.importPath(legacyInfoFile))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &info))
	assert.Equal(t, custom, info.IndexPath)
	assert.Empty(t, info.GeoLitePath)

	// Without a table the export is an empty list.
	raw, err = os.ReadFile(env.importPath(legacyIndicesFile))
	require.NoError(t, err)
	assert.JSONEq(t, "[]", string(raw))
}

func TestLegacyHandoffIsIdempotent(t *testing.T) {
	env := newLegacyTestEnv(t, true)
	createLegacyTable(t, env.deps.db, 1)

	require.NoError(t, env.prepare(nil))
	require.NoError(t, env.prepare(nil))

	assert.Len(t, env.saved, 0, "no legacy value was configured, so nothing is saved")
	assert.Equal(t, 1, env.disabled)
}

func TestLegacyHandoffTriggersOnRowsAloneWithoutTouchingTheFlag(t *testing.T) {
	env := newLegacyTestEnv(t, false)
	createLegacyTable(t, env.deps.db, 1)

	require.NoError(t, env.prepare(nil))

	assert.FileExists(t, env.importPath(legacyIndicesFile))
	assert.Equal(t, 0, env.disabled)
}

func TestLegacyHandoffSkipsNodesThatNeverIndexed(t *testing.T) {
	env := newLegacyTestEnv(t, false)

	// No table at all.
	require.NoError(t, env.prepare(nil))
	assert.NoDirExists(t, filepath.Join(env.dataDir, legacyImportDir))

	// An empty table does not trigger either.
	createLegacyTable(t, env.deps.db, 0)
	require.NoError(t, env.prepare(nil))
	assert.NoDirExists(t, filepath.Join(env.dataDir, legacyImportDir))
	assert.NoFileExists(t, filepath.Join(env.dataDir, legacyDoneMarker))
}

func TestLegacyHandoffKeepsSettingsTheUserAlreadyChose(t *testing.T) {
	env := newLegacyTestEnv(t, true)
	conf, err := ini.Load([]byte("[nginx_log]\nIncrementalIndexInterval = 30\nMaxConcurrentIndexTasks = 3\n"))
	require.NoError(t, err)
	env.deps.conf = conf

	require.NoError(t, env.prepare(map[string]any{"incremental_index_interval": float64(5), "other": "kept"}))

	require.Len(t, env.saved, 1)
	assert.Equal(t, map[string]any{
		"incremental_index_interval": float64(5),
		"max_concurrent_index_tasks": float64(3),
		"other":                      "kept",
	}, env.saved[0])
}

func TestLegacyHandoffFailureLeavesTheNodeAsItWas(t *testing.T) {
	env := newLegacyTestEnv(t, true)
	createLegacyTable(t, env.deps.db, 1)
	conf, err := ini.Load([]byte("[nginx_log]\nIncrementalIndexInterval = 30\n"))
	require.NoError(t, err)
	env.deps.conf = conf

	err = prepareLegacyLogAnalytics(env.deps, env.dataDir, nil, func(map[string]any) error {
		return errors.New("database is locked")
	})
	require.Error(t, err)
	assert.NoDirExists(t, filepath.Join(env.dataDir, legacyImportDir))
	assert.NoFileExists(t, filepath.Join(env.dataDir, legacyDoneMarker))
	assert.Equal(t, 0, env.disabled)

	// The next start tries again and succeeds.
	require.NoError(t, env.prepare(nil))
	assert.FileExists(t, env.importPath(legacyIndicesFile))
	assert.Equal(t, 1, env.disabled)

	// A failing flag switch does not record the handoff as done.
	failing := newLegacyTestEnv(t, true)
	failing.deps.disableIndexing = func() error { return errors.New("read only") }
	require.Error(t, failing.prepare(nil))
	assert.NoFileExists(t, filepath.Join(failing.dataDir, legacyDoneMarker))
}

func TestLegacyHandoffReplacesAnUnconsumedImport(t *testing.T) {
	env := newLegacyTestEnv(t, true)
	createLegacyTable(t, env.deps.db, 1)
	require.NoError(t, os.MkdirAll(filepath.Join(env.dataDir, legacyImportDir), 0o700))
	require.NoError(t, os.WriteFile(env.importPath("stale.json"), []byte("{}"), 0o600))

	require.NoError(t, env.prepare(nil))

	assert.NoFileExists(t, env.importPath("stale.json"))
	assert.FileExists(t, env.importPath(legacyIndicesFile))
}

func TestLegacyLogSettingsReadTheEnvironmentFirst(t *testing.T) {
	conf, err := ini.Load([]byte("[nginx_log]\nIncrementalIndexInterval = 30\nGeoMapPath = maps\nMaxConcurrentIndexTasks = 0\n"))
	require.NoError(t, err)

	deps := legacyDeps{
		conf:    conf,
		confDir: "/etc/nginx-ui",
		env: func(key string) (string, bool) {
			values := map[string]string{
				"NGINX_UI_NGINX_LOG_INCREMENTAL_INDEX_INTERVAL": "45",
				"NGINX_UI_NGINX_LOG_INDEX_CUSTOM_MMDB":          "/data/corp.mmdb",
			}
			value, ok := values[key]
			return value, ok
		},
	}

	assert.Equal(t, map[string]any{
		"incremental_index_interval": float64(45),
		"index_custom_mmdb":          "/data/corp.mmdb",
		"geo_map_path":               filepath.Join("/etc/nginx-ui", "maps"),
	}, legacyLogSettings(deps), "zero and empty values keep the plugin defaults")
}

func TestLegacyLogSettingsWithoutAConfigFile(t *testing.T) {
	assert.Empty(t, legacyLogSettings(legacyDeps{}))
}

func TestCleanupDropsAnEmptyTable(t *testing.T) {
	db := newLegacyTestDB(t)
	createLegacyTable(t, db, 0)

	require.NoError(t, cleanupLegacyLogAnalytics(db, t.TempDir()))
	assert.False(t, db.Migrator().HasTable(legacyIndicesTable))
}

func TestCleanupKeepsRowsUntilThePluginConsumedTheImport(t *testing.T) {
	db := newLegacyTestDB(t)
	createLegacyTable(t, db, 1)
	dataDir := t.TempDir()

	// No handoff yet.
	require.NoError(t, cleanupLegacyLogAnalytics(db, dataDir))
	assert.True(t, db.Migrator().HasTable(legacyIndicesTable))

	// Handed over, the plugin has not imported yet.
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, legacyDoneMarker), nil, 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, legacyImportDir), 0o700))
	require.NoError(t, cleanupLegacyLogAnalytics(db, dataDir))
	assert.True(t, db.Migrator().HasTable(legacyIndicesTable))

	// The plugin removed the import directory.
	require.NoError(t, os.RemoveAll(filepath.Join(dataDir, legacyImportDir)))
	require.NoError(t, cleanupLegacyLogAnalytics(db, dataDir))
	assert.False(t, db.Migrator().HasTable(legacyIndicesTable))
	assert.NoFileExists(t, filepath.Join(dataDir, legacyDoneMarker))

	// Nothing to do once the table is gone.
	require.NoError(t, cleanupLegacyLogAnalytics(db, dataDir))
}

func TestCleanupKeepsRowsWhenThereWasNoHandoff(t *testing.T) {
	db := newLegacyTestDB(t)
	createLegacyTable(t, db, 1)

	// The import directory is missing but nothing was ever handed over, for
	// example a node that indexed before and has not installed the plugin.
	require.NoError(t, cleanupLegacyLogAnalytics(db, t.TempDir()))
	assert.True(t, db.Migrator().HasTable(legacyIndicesTable))
}

func TestPrepareLegacyLogAnalyticsIgnoresOtherPlugins(t *testing.T) {
	dataDir := t.TempDir()
	called := false
	require.NoError(t, PrepareLegacyLogAnalytics("com.example.other", dataDir, nil, func(map[string]any) error {
		called = true
		return nil
	}))
	assert.False(t, called)
	entries, err := os.ReadDir(dataDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
