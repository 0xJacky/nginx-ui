package site

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSiteMutationTest(t *testing.T) (string, func()) {
	t.Helper()

	confDir := t.TempDir()
	for _, dir := range []string{"sites-available", "sites-enabled"} {
		if err := os.MkdirAll(filepath.Join(confDir, dir), 0o755); err != nil {
			t.Fatalf("failed to create %s: %v", dir, err)
		}
	}

	originalConfigDir := appsettings.NginxSettings.ConfigDir
	originalPIDPath := appsettings.NginxSettings.PIDPath
	originalReloadCmd := appsettings.NginxSettings.ReloadCmd
	originalRestartCmd := appsettings.NginxSettings.RestartCmd
	originalTestConfigCmd := appsettings.NginxSettings.TestConfigCmd

	appsettings.NginxSettings.ConfigDir = confDir
	appsettings.NginxSettings.PIDPath = filepath.Join(confDir, "nginx.pid")
	appsettings.NginxSettings.ReloadCmd = "true"
	appsettings.NginxSettings.RestartCmd = "true"
	appsettings.NginxSettings.TestConfigCmd = "true"
	if err := os.WriteFile(appsettings.NginxSettings.PIDPath, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatalf("failed to seed nginx pid file: %v", err)
	}

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.Site{}, &model.ConfigBackup{}, &model.LLMSession{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	syncQueryCompleted := make(chan struct{}, 1)
	if err := db.Callback().Query().After("gorm:query").Register("test:site_sync_query_completed", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "sites" {
			select {
			case syncQueryCompleted <- struct{}{}:
			default:
			}
		}
	}); err != nil {
		t.Fatalf("failed to register site sync query callback: %v", err)
	}

	t.Cleanup(func() {
		appsettings.NginxSettings.ConfigDir = originalConfigDir
		appsettings.NginxSettings.PIDPath = originalPIDPath
		appsettings.NginxSettings.ReloadCmd = originalReloadCmd
		appsettings.NginxSettings.RestartCmd = originalRestartCmd
		appsettings.NginxSettings.TestConfigCmd = originalTestConfigCmd
	})

	waitForSyncQuery := func() {
		t.Helper()

		select {
		case <-syncQueryCompleted:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for site sync query")
		}
	}

	return confDir, waitForSyncQuery
}

func TestSaveAllowsManagedSiteHostname(t *testing.T) {
	confDir, waitForSyncQuery := setupSiteMutationTest(t)

	err := Save("example.com", "server {\n    listen 80;\n}\n", true, 0, nil, "")
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	waitForSyncQuery()

	if _, err := os.Stat(filepath.Join(confDir, "sites-available", "example.com")); err != nil {
		t.Fatalf("expected saved site file: %v", err)
	}
}

func TestSaveRejectsDangerousSiteExtension(t *testing.T) {
	setupSiteMutationTest(t)

	err := Save("evil.pl", "server {\n}\n", true, 0, nil, "")
	if err == nil {
		t.Fatal("Save expected validation error")
	}
	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("Save expected cosy error, got %v", err)
	}
}

func TestRenameAllowsManagedSiteHostname(t *testing.T) {
	confDir, waitForSyncQuery := setupSiteMutationTest(t)

	if err := os.WriteFile(filepath.Join(confDir, "sites-available", "old.example.com"), []byte("server {\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed site config: %v", err)
	}

	err := Rename("old.example.com", "new.example.com")
	if err != nil {
		t.Fatalf("Rename returned error: %v", err)
	}
	waitForSyncQuery()

	if _, err := os.Stat(filepath.Join(confDir, "sites-available", "new.example.com")); err != nil {
		t.Fatalf("expected renamed site file: %v", err)
	}
}

func TestRenameRejectsDangerousSiteExtension(t *testing.T) {
	confDir, _ := setupSiteMutationTest(t)

	if err := os.WriteFile(filepath.Join(confDir, "sites-available", "old.example.com"), []byte("server {\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed site config: %v", err)
	}

	err := Rename("old.example.com", "evil.pl")
	if err == nil {
		t.Fatal("Rename expected validation error")
	}
	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("Rename expected cosy error, got %v", err)
	}
}

func TestDuplicateRejectsDangerousSiteExtension(t *testing.T) {
	confDir, _ := setupSiteMutationTest(t)

	if err := os.WriteFile(filepath.Join(confDir, "sites-available", "source.example.com"), []byte("server {\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed site config: %v", err)
	}

	err := Duplicate("source.example.com", "copy.pl")
	if err == nil {
		t.Fatal("Duplicate expected validation error")
	}
	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("Duplicate expected cosy error, got %v", err)
	}
}

func TestDuplicateRejectsBinarySiteContent(t *testing.T) {
	confDir, _ := setupSiteMutationTest(t)

	if err := os.WriteFile(filepath.Join(confDir, "sites-available", "source.example.com"), []byte{0xff, 0xfe, 0xfd}, 0o644); err != nil {
		t.Fatalf("failed to seed site config: %v", err)
	}

	err := Duplicate("source.example.com", "copy.example.com")
	if err == nil {
		t.Fatal("Duplicate expected validation error")
	}
	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("Duplicate expected cosy error, got %v", err)
	}
}

// A regular file in sites-enabled (copied config, restored backup) used to be
// re-linked on rename just like a symlink. Gating on symlink mode alone left
// the stale file serving the old content under the old name.
func TestRenameRelinksRegularFileInEnabledDir(t *testing.T) {
	confDir, waitForSyncQuery := setupSiteMutationTest(t)

	availableDir := filepath.Join(confDir, "sites-available")
	enabledDir := filepath.Join(confDir, "sites-enabled")
	if err := os.WriteFile(filepath.Join(availableDir, "old.example.com"), []byte("server {\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed site config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(enabledDir, "old.example.com"), []byte("server {\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed enabled copy: %v", err)
	}

	if err := Rename("old.example.com", "new.example.com"); err != nil {
		t.Fatalf("Rename returned error: %v", err)
	}
	waitForSyncQuery()

	if _, err := os.Lstat(filepath.Join(enabledDir, "old.example.com")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected the stale enabled file to be removed, got %v", err)
	}
	info, err := os.Lstat(filepath.Join(enabledDir, "new.example.com"))
	if err != nil {
		t.Fatalf("expected relinked enabled entry: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected enabled entry to be a symlink, got mode %v", info.Mode())
	}
	target, err := os.Readlink(filepath.Join(enabledDir, "new.example.com"))
	if err != nil {
		t.Fatalf("failed to read relinked entry: %v", err)
	}
	if target != filepath.Join(availableDir, "new.example.com") {
		t.Fatalf("relinked entry points at %q, want the renamed available file", target)
	}
}
