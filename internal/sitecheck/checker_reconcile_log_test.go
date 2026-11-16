package sitecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// capturedLogs records which level each reconcile message was emitted at so the
// tests can assert the logging cadence without touching cosy's global logger.
type capturedLogs struct {
	mu    sync.Mutex
	warns []string
	infos []string
	debug []string
}

func (c *capturedLogs) format(dst *[]string) func(string, ...any) {
	return func(format string, args ...any) {
		c.mu.Lock()
		defer c.mu.Unlock()
		*dst = append(*dst, fmt.Sprintf(format, args...))
	}
}

// countContaining returns how many captured lines contain substr.
func countContaining(lines []string, substr string) int {
	n := 0
	for _, line := range lines {
		if strings.Contains(line, substr) {
			n++
		}
	}
	return n
}

// installReconcileLogCapture swaps the injectable reconcile sinks for capturing
// closures and resets the remembered report state, restoring both on cleanup so
// the shared package-level state does not leak between tests.
func installReconcileLogCapture(t *testing.T) *capturedLogs {
	t.Helper()

	logs := &capturedLogs{}

	origWarnf := reconcileWarnf
	origDebugf := reconcileDebugf
	origInfof := reconcileInfof

	reconcileWarnf = logs.format(&logs.warns)
	reconcileInfof = logs.format(&logs.infos)
	reconcileDebugf = logs.format(&logs.debug)

	reconcileReportMu.Lock()
	origUnresolved := lastUnresolvedReported
	origRemaining := lastRemainingEmptyReported
	lastUnresolvedReported = -1
	lastRemainingEmptyReported = -1
	reconcileReportMu.Unlock()

	t.Cleanup(func() {
		reconcileWarnf = origWarnf
		reconcileDebugf = origDebugf
		reconcileInfof = origInfof

		reconcileReportMu.Lock()
		lastUnresolvedReported = origUnresolved
		lastRemainingEmptyReported = origRemaining
		reconcileReportMu.Unlock()
	})

	return logs
}

// setupOrphanReconcileEnv wires a file-backed sqlite database and an empty
// config dir so site_configs seeded with an empty site_index can never be
// matched to a current site, keeping them unresolved on every reconcile pass.
func setupOrphanReconcileEnv(t *testing.T) *gorm.DB {
	t.Helper()

	confDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(confDir, "sites-available"), 0o755); err != nil {
		t.Fatalf("failed to create sites-available: %v", err)
	}

	originalConfigDir := settings.NginxSettings.ConfigDir
	originalDB := model.UseDB()
	originalSite := query.Site
	originalSiteConfig := query.SiteConfig
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		model.Use(originalDB)
		query.Site = originalSite
		query.SiteConfig = originalSiteConfig
	})
	settings.NginxSettings.ConfigDir = confDir

	dsn := "file:" + filepath.Join(t.TempDir(), "reconcile.db") + "?_busy_timeout=200"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.AutoMigrate(&model.Site{}, &model.SiteConfig{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	model.Use(db)
	query.Site = &query.Use(db).Site
	query.SiteConfig = &query.Use(db).SiteConfig

	return db
}

// seedOrphanSiteConfig inserts a site_config with no site_name and an empty
// site_index whose host matches no indexed site, so every reconcile pass leaves
// it unresolved.
func seedOrphanSiteConfig(t *testing.T, db *gorm.DB, host string) {
	t.Helper()
	cfg := &model.SiteConfig{Host: host, Port: 80, Scheme: "http"}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatalf("failed to seed orphan site config: %v", err)
	}
}

// TestReconcileSiteConfigSiteIDsSuppressesRepeatedUnresolvedWarn is the
// regression for #2118: a stable orphaned row must warn once on the first pass
// and then stay quiet (Debug) on the next identical pass instead of warning
// every interval forever.
func TestReconcileSiteConfigSiteIDsSuppressesRepeatedUnresolvedWarn(t *testing.T) {
	db := setupOrphanReconcileEnv(t)
	seedOrphanSiteConfig(t, db, "orphan.example.com:80")

	logs := installReconcileLogCapture(t)

	reconcileSiteConfigSiteIDsIfDirty()
	firstWarns := countContaining(logs.warns, "unresolved=1")
	if firstWarns != 1 {
		t.Fatalf("first pass: unresolved WARN count = %d, want 1 (warns=%v)", firstWarns, logs.warns)
	}

	reconcileSiteConfigSiteIDsIfDirty()
	secondWarns := countContaining(logs.warns, "unresolved=1")
	if secondWarns != 1 {
		t.Fatalf("second pass: cumulative unresolved WARN count = %d, want 1 (steady state must not re-warn) (warns=%v)", secondWarns, logs.warns)
	}

	// The second pass must still record the information at Debug so it remains
	// available when debug logging is on.
	if countContaining(logs.debug, "unresolved=1") < 1 {
		t.Fatalf("second pass: expected a Debug line for the unchanged unresolved count, debug=%v", logs.debug)
	}

	// The "remaining empty" WARN must follow the same cadence: once, not twice.
	if got := countContaining(logs.warns, "remaining=1"); got != 1 {
		t.Fatalf("remaining-empty WARN count = %d, want 1 (warns=%v)", got, logs.warns)
	}
}

// TestReconcileSiteConfigSiteIDsWarnsAgainWhenCountChanges asserts operators
// still see a WARN the first time and whenever the unresolved count changes.
func TestReconcileSiteConfigSiteIDsWarnsAgainWhenCountChanges(t *testing.T) {
	db := setupOrphanReconcileEnv(t)
	for i := 0; i < 5; i++ {
		seedOrphanSiteConfig(t, db, fmt.Sprintf("orphan-%d.example.com:80", i))
	}

	logs := installReconcileLogCapture(t)

	// First observation: 5 unresolved -> WARN.
	reconcileSiteConfigSiteIDsIfDirty()
	if got := countContaining(logs.warns, "unresolved=5"); got != 1 {
		t.Fatalf("first pass: unresolved=5 WARN count = %d, want 1 (warns=%v)", got, logs.warns)
	}

	// Unchanged pass: still 5 -> no new WARN.
	reconcileSiteConfigSiteIDsIfDirty()
	if got := countContaining(logs.warns, "unresolved=5"); got != 1 {
		t.Fatalf("unchanged pass: unresolved=5 WARN count = %d, want 1 (warns=%v)", got, logs.warns)
	}

	// Change the count to 3 by removing two orphans; the new value must WARN.
	if err := db.Unscoped().Where("host = ? OR host = ?", "orphan-0.example.com:80", "orphan-1.example.com:80").
		Delete(&model.SiteConfig{}).Error; err != nil {
		t.Fatalf("failed to delete orphan rows: %v", err)
	}

	reconcileSiteConfigSiteIDsIfDirty()
	if got := countContaining(logs.warns, "unresolved=3"); got != 1 {
		t.Fatalf("changed pass: unresolved=3 WARN count = %d, want 1 (warns=%v)", got, logs.warns)
	}
}
