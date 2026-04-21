package stream

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

func setupStreamMutationTest(t *testing.T) string {
	t.Helper()

	confDir := t.TempDir()
	for _, dir := range []string{"streams-available", "streams-enabled"} {
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

	if err := db.AutoMigrate(&model.Stream{}, &model.ConfigBackup{}, &model.LLMSession{}); err != nil {
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

func TestSaveAllowsManagedStreamName(t *testing.T) {
	confDir := setupStreamMutationTest(t)

	err := Save("tcp_proxy", "server {\n    listen 8080;\n}\n", true, nil, "")
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(confDir, "streams-available", "tcp_proxy")); err != nil {
		t.Fatalf("expected saved stream file: %v", err)
	}
}

func TestSaveRejectsDangerousStreamExtension(t *testing.T) {
	setupStreamMutationTest(t)

	err := Save("evil.sh", "server {\n}\n", true, nil, "")
	if err == nil {
		t.Fatal("Save expected validation error")
	}
	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("Save expected cosy error, got %v", err)
	}
}

func TestRenameAllowsManagedStreamName(t *testing.T) {
	confDir := setupStreamMutationTest(t)

	if err := os.WriteFile(filepath.Join(confDir, "streams-available", "tcp_proxy"), []byte("server {\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed stream config: %v", err)
	}

	err := Rename("tcp_proxy", "tcp_proxy_new")
	if err != nil {
		t.Fatalf("Rename returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(confDir, "streams-available", "tcp_proxy_new")); err != nil {
		t.Fatalf("expected renamed stream file: %v", err)
	}
}

func TestRenameRejectsDangerousStreamExtension(t *testing.T) {
	confDir := setupStreamMutationTest(t)

	if err := os.WriteFile(filepath.Join(confDir, "streams-available", "tcp_proxy"), []byte("server {\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed stream config: %v", err)
	}

	err := Rename("tcp_proxy", "evil.sh")
	if err == nil {
		t.Fatal("Rename expected validation error")
	}
	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("Rename expected cosy error, got %v", err)
	}
}

func TestDuplicateRejectsDangerousStreamExtension(t *testing.T) {
	confDir := setupStreamMutationTest(t)

	if err := os.WriteFile(filepath.Join(confDir, "streams-available", "tcp_proxy"), []byte("server {\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed stream config: %v", err)
	}

	err := Duplicate("tcp_proxy", "copy.sh")
	if err == nil {
		t.Fatal("Duplicate expected validation error")
	}
	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("Duplicate expected cosy error, got %v", err)
	}
}

func TestDuplicateRejectsBinaryStreamContent(t *testing.T) {
	confDir := setupStreamMutationTest(t)

	if err := os.WriteFile(filepath.Join(confDir, "streams-available", "tcp_proxy"), []byte{0xff, 0xfe, 0xfd}, 0o644); err != nil {
		t.Fatalf("failed to seed stream config: %v", err)
	}

	err := Duplicate("tcp_proxy", "copy_proxy")
	if err == nil {
		t.Fatal("Duplicate expected validation error")
	}
	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("Duplicate expected cosy error, got %v", err)
	}
}
