package sitecheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestUpgradeSiteConfigAssociationsCreatesMissingSitesInTx covers sites that
// exist in sites-available but have no sites row yet. The upgrade must create
// that row inside its own transaction; an insert through another pooled
// connection self-deadlocks on SQLite until busy_timeout and leaves the record
// unresolved on every start.
func TestUpgradeSiteConfigAssociationsCreatesMissingSitesInTx(t *testing.T) {
	confDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(confDir, "sites-available"), 0o755); err != nil {
		t.Fatalf("failed to create sites-available: %v", err)
	}

	originalConfigDir := settings.NginxSettings.ConfigDir
	originalDB := model.UseDB()
	originalSite := query.Site
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		model.Use(originalDB)
		query.Site = originalSite
	})
	settings.NginxSettings.ConfigDir = confDir

	// A file-backed database is required: only separate connections to a real
	// file reproduce the lock contention. The short busy timeout keeps a
	// regression from stalling the test for the 5s driver default.
	dsn := "file:" + filepath.Join(t.TempDir(), "upgrade.db") + "?_busy_timeout=200"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
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

	legacy := &model.SiteConfig{SiteName: "example.conf", Host: "example.com:80", Port: 80, Scheme: "http"}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("failed to seed site config: %v", err)
	}

	updated, _, unresolved, err := upgradeSiteConfigAssociations()
	if err != nil {
		t.Fatalf("upgradeSiteConfigAssociations returned error: %v", err)
	}
	if unresolved != 0 {
		t.Fatalf("unresolved = %d, want 0", unresolved)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1", updated)
	}

	var created model.Site
	wantPath := filepath.Join(confDir, "sites-available", "example.conf")
	if err := db.Where("path = ?", wantPath).First(&created).Error; err != nil {
		t.Fatalf("sites row for %s was not created: %v", wantPath, err)
	}

	var got model.SiteConfig
	if err := db.First(&got, legacy.ID).Error; err != nil {
		t.Fatalf("failed to reload site config: %v", err)
	}
	if got.SiteIndex != created.ID || got.SiteID != created.ID {
		t.Fatalf("site config index = %d/%d, want %d", got.SiteIndex, got.SiteID, created.ID)
	}
}
