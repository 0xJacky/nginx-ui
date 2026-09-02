package site

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupSiteStatusTest(t *testing.T) (*gorm.DB, *bytes.Buffer, string) {
	t.Helper()

	confDir := t.TempDir()
	for _, dir := range []string{"sites-available", "sites-enabled"} {
		if err := os.MkdirAll(filepath.Join(confDir, dir), 0o755); err != nil {
			t.Fatalf("failed to create %s: %v", dir, err)
		}
	}

	originalConfigDir := appsettings.NginxSettings.ConfigDir
	appsettings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		appsettings.NginxSettings.ConfigDir = originalConfigDir
	})

	var logs bytes.Buffer
	database, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{Logger: gormlogger.New(log.New(&logs, "", 0), gormlogger.Config{
			SlowThreshold: time.Second,
			LogLevel:      gormlogger.Info,
			Colorful:      false,
		})},
	)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := database.AutoMigrate(&model.Namespace{}, &model.Site{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	query.SetDefault(database)
	logs.Reset()
	return database, &logs, confDir
}

func TestGetSiteStatusDoesNotLogMissingOptionalSiteRecord(t *testing.T) {
	_, logs, _ := setupSiteStatusTest(t)

	if got := GetSiteStatus("nginx-ui.conf.bak.20260626210626"); got != StatusDisabled {
		t.Fatalf("got %q, want %q", got, StatusDisabled)
	}
	if strings.Contains(logs.String(), "record not found") {
		t.Fatalf("optional site lookup logged an expected missing record:\n%s", logs.String())
	}
}

func TestGetSiteStatusUsesRemoteDeploymentIntent(t *testing.T) {
	database, _, confDir := setupSiteStatusTest(t)

	namespace := &model.Namespace{DeployMode: model.DeployModeRemote}
	if err := database.Create(namespace).Error; err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	siteModel := &model.Site{
		Path:          filepath.Join(confDir, "sites-available", "remote.conf"),
		NamespaceID:   namespace.ID,
		RemoteEnabled: true,
	}
	if err := database.Create(siteModel).Error; err != nil {
		t.Fatalf("failed to create site: %v", err)
	}

	if got := GetSiteStatus("remote.conf"); got != StatusEnabled {
		t.Fatalf("got %q, want %q", got, StatusEnabled)
	}
}
