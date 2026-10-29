package sitecheck

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestGetOrCreateSiteConfigForURLDoesNotLogExpectedMisses covers #2036: the
// key, legacy key and host lookups miss for a URL seen for the first time,
// and must not print GORM's "record not found" trace for each of them.
func TestGetOrCreateSiteConfigForURLDoesNotLogExpectedMisses(t *testing.T) {
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

	var logs bytes.Buffer
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "misses.db")), &gorm.Config{
		Logger: logger.New(log.New(&logs, "", 0), logger.Config{LogLevel: logger.Info}),
	})
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
	q := query.Use(db)
	query.Site = &q.Site
	query.SiteConfig = &q.SiteConfig
	logs.Reset()

	config := getOrCreateSiteConfigForURL("quiet.conf", "https://www.quiet.example:443")
	if config == nil || config.Host != "www.quiet.example:443" {
		t.Fatalf("got config %+v, want one for www.quiet.example:443", config)
	}
	if strings.Contains(logs.String(), "record not found") {
		t.Fatalf("expected lookup misses were logged:\n%s", logs.String())
	}
}
