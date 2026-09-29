package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/google/uuid"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
	"gopkg.in/ini.v1"
	"gorm.io/gorm"
)

// This file hands the log analytics data of a node that kept it inside the
// host over to the official plugin. It only exists for the upgrade from the
// releases that had advanced indexing built in and goes away with the next
// major version.
//
// Before the plugin starts for the first time, the host
//   - writes the old indexing settings into the plugin settings,
//   - exports the rows of the nginx_log_indices table to
//     <data_dir>/import/nginx_log_indices.json,
//   - describes the old index and city database in <data_dir>/import/legacy.json,
//   - switches the host side flag IndexingEnabled off.
//
// The plugin imports the rows, adopts the files and removes import/. Once that
// happened the host drops the old table. Every step can run again after a
// failure.

const (
	// OfficialLogAnalyticsPluginID is the plugin that took over log analytics.
	OfficialLogAnalyticsPluginID = "com.nginxui.log-analytics"

	legacyIndicesTable   = "nginx_log_indices"
	legacyImportDir      = "import"
	legacyIndicesFile    = "nginx_log_indices.json"
	legacyInfoFile       = "legacy.json"
	legacyDoneMarker     = ".legacy-handoff-done"
	legacyDefaultIndex   = "log-index"
	legacyDefaultCityMMD = "GeoLite2-City.mmdb"
	legacyEnvPrefix      = "NGINX_UI_NGINX_LOG_"
)

// legacyMu serialises the handoff and the table cleanup, which can run at the
// same time while the host boots.
var legacyMu sync.Mutex

// legacyLogIndexRow mirrors the columns of the old nginx_log_indices table. The
// JSON names are the ones the plugin reads.
type legacyLogIndexRow struct {
	ID             uuid.UUID  `gorm:"column:id" json:"id"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at" json:"updated_at"`
	Path           string     `gorm:"column:path" json:"path"`
	MainLogPath    string     `gorm:"column:main_log_path" json:"main_log_path"`
	LastModified   time.Time  `gorm:"column:last_modified" json:"last_modified"`
	LastSize       int64      `gorm:"column:last_size" json:"last_size"`
	LastPosition   int64      `gorm:"column:last_position" json:"last_position"`
	LastIndexed    time.Time  `gorm:"column:last_indexed" json:"last_indexed"`
	IndexStartTime *time.Time `gorm:"column:index_start_time" json:"index_start_time"`
	IndexDuration  *int64     `gorm:"column:index_duration" json:"index_duration"`
	TimeRangeStart *time.Time `gorm:"column:time_range_start" json:"timerange_start"`
	TimeRangeEnd   *time.Time `gorm:"column:time_range_end" json:"timerange_end"`
	DocumentCount  uint64     `gorm:"column:document_count" json:"document_count"`
	Enabled        bool       `gorm:"column:enabled" json:"enabled"`
	IndexStatus    string     `gorm:"column:index_status" json:"index_status"`
	ErrorMessage   string     `gorm:"column:error_message" json:"error_message,omitempty"`
	ErrorTime      *time.Time `gorm:"column:error_time" json:"error_time,omitempty"`
	RetryCount     int        `gorm:"column:retry_count" json:"retry_count"`
	QueuePosition  int        `gorm:"column:queue_position" json:"queue_position,omitempty"`
}

// legacyInfo is the content of import/legacy.json.
type legacyInfo struct {
	IndexPath   string `json:"index_path"`
	GeoLitePath string `json:"geolite_path"`
}

// legacyDeps gathers what the handoff reads from the host, so tests can
// replace it.
type legacyDeps struct {
	db *gorm.DB
	// conf is the parsed app.ini, which still holds the keys the settings
	// structs no longer have.
	conf *ini.File
	// confDir is the directory holding app.ini.
	confDir string
	// env looks an environment variable up.
	env func(key string) (string, bool)
	// indexingEnabled and indexPath are the two settings the host keeps.
	indexingEnabled bool
	indexPath       string
	// disableIndexing persists IndexingEnabled=false.
	disableIndexing func() error
}

func defaultLegacyDeps() legacyDeps {
	return legacyDeps{
		db:              model.UseDB(),
		conf:            cSettings.Conf,
		confDir:         filepath.Dir(cSettings.ConfPath),
		env:             os.LookupEnv,
		indexingEnabled: settings.NginxLogSettings.IndexingEnabled,
		indexPath:       settings.NginxLogSettings.IndexPath,
		disableIndexing: func() error {
			return settings.Update(func() { settings.NginxLogSettings.IndexingEnabled = false })
		},
	}
}

// PrepareLegacyLogAnalytics performs the handoff for the log analytics plugin.
// It does nothing for another plugin, for a node that never used the advanced
// indexing and once the handoff is done. stored holds the current plugin
// settings, persist saves the settings the handoff adds and is called before
// any file is written. An error leaves the node as it was, the next start
// tries again.
func PrepareLegacyLogAnalytics(pluginID, dataDir string, stored map[string]any, persist func(map[string]any) error) error {
	if pluginID != OfficialLogAnalyticsPluginID {
		return nil
	}
	return prepareLegacyLogAnalytics(defaultLegacyDeps(), dataDir, stored, persist)
}

func prepareLegacyLogAnalytics(deps legacyDeps, dataDir string, stored map[string]any, persist func(map[string]any) error) error {
	legacyMu.Lock()
	defer legacyMu.Unlock()

	if _, err := os.Stat(filepath.Join(dataDir, legacyDoneMarker)); err == nil {
		return nil
	}

	rows, err := exportLegacyRows(deps.db)
	if err != nil {
		return fmt.Errorf("export the old log index records: %w", err)
	}
	if !deps.indexingEnabled && len(rows) == 0 {
		return nil
	}

	logger.Infof("Handing the log analytics of this node over to the plugin %s", OfficialLogAnalyticsPluginID)

	merged, changed := mergeLegacySettings(stored, legacyLogSettings(deps))
	if changed {
		if err = persist(merged); err != nil {
			return fmt.Errorf("save the plugin settings: %w", err)
		}
	}

	info := legacyInfo{
		IndexPath:   legacyIndexPath(deps),
		GeoLitePath: legacyGeoLitePath(deps),
	}
	if err = writeLegacyImport(dataDir, rows, info); err != nil {
		return fmt.Errorf("write the handoff: %w", err)
	}

	if deps.indexingEnabled {
		if err = deps.disableIndexing(); err != nil {
			return fmt.Errorf("switch the old indexing off: %w", err)
		}
	}

	if err = os.WriteFile(filepath.Join(dataDir, legacyDoneMarker), nil, 0o600); err != nil {
		return fmt.Errorf("record the handoff: %w", err)
	}
	return nil
}

// CleanupLegacyLogAnalytics drops the old nginx_log_indices table once nothing
// needs it: it is empty, or the handoff happened and the plugin removed the
// import directory, which is how it reports a finished import. Until then the
// table stays so the handoff can be repeated.
func CleanupLegacyLogAnalytics(_ context.Context) {
	deps := defaultLegacyDeps()
	dataDir := GetManager().DataDir(OfficialLogAnalyticsPluginID)
	if err := cleanupLegacyLogAnalytics(deps.db, dataDir); err != nil {
		logger.Warnf("Clean up the old log index table: %v", err)
	}
}

func cleanupLegacyLogAnalytics(db *gorm.DB, dataDir string) error {
	if db == nil {
		return nil
	}

	legacyMu.Lock()
	defer legacyMu.Unlock()

	if !db.Migrator().HasTable(legacyIndicesTable) {
		return nil
	}

	var count int64
	if err := db.Table(legacyIndicesTable).Count(&count).Error; err != nil {
		return err
	}

	markerPath := filepath.Join(dataDir, legacyDoneMarker)
	if count > 0 {
		if _, err := os.Stat(markerPath); err != nil {
			return nil
		}
		if _, err := os.Stat(filepath.Join(dataDir, legacyImportDir)); err == nil {
			return nil
		}
	}

	if err := db.Migrator().DropTable(legacyIndicesTable); err != nil {
		return err
	}
	_ = os.Remove(markerPath)
	logger.Info("Dropped the old log index table, its content lives in the log analytics plugin")
	return nil
}

// exportLegacyRows reads the old table with raw SQL. A missing table yields no
// rows.
func exportLegacyRows(db *gorm.DB) ([]legacyLogIndexRow, error) {
	rows := []legacyLogIndexRow{}
	if db == nil || !db.Migrator().HasTable(legacyIndicesTable) {
		return rows, nil
	}
	if err := db.Raw("SELECT * FROM " + legacyIndicesTable).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// legacyLogSettings collects the four settings the host dropped. The values
// come from the raw app.ini section, an environment variable of the old name
// wins like it did for the settings structs. Relative paths were resolved
// against the directory of app.ini, so they are made absolute here.
func legacyLogSettings(deps legacyDeps) map[string]any {
	lookup := func(iniKey, envKey string) string {
		if deps.env != nil {
			if value, ok := deps.env(legacyEnvPrefix + envKey); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
		if deps.conf == nil {
			return ""
		}
		section, err := deps.conf.GetSection("nginx_log")
		if err != nil || !section.HasKey(iniKey) {
			return ""
		}
		return strings.TrimSpace(section.Key(iniKey).String())
	}

	values := map[string]any{}
	if number, err := strconv.Atoi(lookup("IncrementalIndexInterval", "INCREMENTAL_INDEX_INTERVAL")); err == nil && number > 0 {
		values["incremental_index_interval"] = float64(number)
	}
	if number, err := strconv.Atoi(lookup("MaxConcurrentIndexTasks", "MAX_CONCURRENT_INDEX_TASKS")); err == nil && number > 0 {
		values["max_concurrent_index_tasks"] = float64(number)
	}
	if path := lookup("IndexCustomMMDB", "INDEX_CUSTOM_MMDB"); path != "" {
		values["index_custom_mmdb"] = absoluteLegacyPath(deps.confDir, path)
	}
	if path := lookup("GeoMapPath", "GEO_MAP_PATH"); path != "" {
		values["geo_map_path"] = absoluteLegacyPath(deps.confDir, path)
	}
	return values
}

func absoluteLegacyPath(confDir, path string) string {
	if filepath.IsAbs(path) || confDir == "" || confDir == "." {
		return path
	}
	return filepath.Join(confDir, path)
}

// mergeLegacySettings adds the legacy values the plugin has no value for yet,
// so a value the user set in the plugin is never overwritten.
func mergeLegacySettings(stored, legacy map[string]any) (map[string]any, bool) {
	merged := make(map[string]any, len(stored)+len(legacy))
	for key, value := range stored {
		merged[key] = value
	}
	changed := false
	for key, value := range legacy {
		if _, exists := merged[key]; exists {
			continue
		}
		merged[key] = value
		changed = true
	}
	return merged, changed
}

// legacyIndexPath is the directory the host kept its index in: the configured
// path, else log-index next to app.ini. It is empty when there is none.
func legacyIndexPath(deps legacyDeps) string {
	candidates := []string{}
	if path := strings.TrimSpace(deps.indexPath); path != "" {
		candidates = append(candidates, path)
	}
	if deps.confDir != "" {
		candidates = append(candidates, filepath.Join(deps.confDir, legacyDefaultIndex))
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	return ""
}

// legacyGeoLitePath is the city database the host downloaded next to app.ini.
// A database named by IndexCustomMMDB stays where it is, the plugin setting
// points at it.
func legacyGeoLitePath(deps legacyDeps) string {
	if deps.confDir == "" {
		return ""
	}
	path := filepath.Join(deps.confDir, legacyDefaultCityMMD)
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		return path
	}
	return ""
}

// writeLegacyImport replaces <dataDir>/import with the handoff files. They are
// written aside first, so the plugin never sees a half written directory.
func writeLegacyImport(dataDir string, rows []legacyLogIndexRow, info legacyInfo) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}

	staged, err := os.MkdirTemp(dataDir, ".import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staged)

	if err = writeJSONFile(filepath.Join(staged, legacyIndicesFile), rows); err != nil {
		return err
	}
	if err = writeJSONFile(filepath.Join(staged, legacyInfoFile), info); err != nil {
		return err
	}

	target := filepath.Join(dataDir, legacyImportDir)
	if err = os.RemoveAll(target); err != nil {
		return err
	}
	if err = os.Rename(staged, target); err != nil {
		return err
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

// prepareLegacyLogAnalytics runs the handoff for an entry before its process
// is built, so the plugin finds its settings and the import directory on its
// first start. A failure is logged and retried at the next start.
func (m *Manager) prepareLegacyLogAnalytics(item *entry) {
	if item.id != OfficialLogAnalyticsPluginID {
		return
	}

	m.mu.RLock()
	row, dataDir := item.row, item.dataDir
	var stored map[string]any
	if row != nil {
		stored = row.Settings
	}
	m.mu.RUnlock()
	if row == nil {
		return
	}

	err := PrepareLegacyLogAnalytics(item.id, dataDir, stored, func(values map[string]any) error {
		m.mu.Lock()
		row.Settings = values
		m.mu.Unlock()
		return m.saveRow(context.Background(), row)
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		m.log.Warnf("[plugin:%s] hand over the old log analytics: %v", item.id, err)
	}
}
