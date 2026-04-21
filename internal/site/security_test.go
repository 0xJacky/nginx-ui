package site

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSiteMutationTest(t *testing.T) string {
	t.Helper()

	confDir := t.TempDir()
	for _, dir := range []string{"sites-available", "sites-enabled"} {
		if err := os.MkdirAll(filepath.Join(confDir, dir), 0o755); err != nil {
			t.Fatalf("failed to create %s: %v", dir, err)
		}
	}

	originalConfigDir := appsettings.NginxSettings.ConfigDir
	originalReloadCmd := appsettings.NginxSettings.ReloadCmd
	originalRestartCmd := appsettings.NginxSettings.RestartCmd
	originalTestConfigCmd := appsettings.NginxSettings.TestConfigCmd

	appsettings.NginxSettings.ConfigDir = confDir
	appsettings.NginxSettings.ReloadCmd = "true"
	appsettings.NginxSettings.RestartCmd = "true"
	appsettings.NginxSettings.TestConfigCmd = "true"

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

	t.Cleanup(func() {
		appsettings.NginxSettings.ConfigDir = originalConfigDir
		appsettings.NginxSettings.ReloadCmd = originalReloadCmd
		appsettings.NginxSettings.RestartCmd = originalRestartCmd
		appsettings.NginxSettings.TestConfigCmd = originalTestConfigCmd
	})

	return confDir
}

func TestSaveAllowsManagedSiteHostname(t *testing.T) {
	confDir := setupSiteMutationTest(t)

	err := Save("example.com", "server {\n    listen 80;\n}\n", true, 0, nil, "")
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

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
	confDir := setupSiteMutationTest(t)

	if err := os.WriteFile(filepath.Join(confDir, "sites-available", "old.example.com"), []byte("server {\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed site config: %v", err)
	}

	err := Rename("old.example.com", "new.example.com")
	if err != nil {
		t.Fatalf("Rename returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(confDir, "sites-available", "new.example.com")); err != nil {
		t.Fatalf("expected renamed site file: %v", err)
	}
}

func TestRenameRejectsDangerousSiteExtension(t *testing.T) {
	confDir := setupSiteMutationTest(t)

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
	confDir := setupSiteMutationTest(t)

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
	confDir := setupSiteMutationTest(t)

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
