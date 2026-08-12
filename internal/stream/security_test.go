package stream

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

func setupStreamMutationTest(t *testing.T) (string, func()) {
	t.Helper()

	confDir := t.TempDir()
	for _, dir := range []string{"streams-available", "streams-enabled"} {
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
	// Seed a live PID so nginx.Reload takes the reload path instead of falling
	// back to a restart, which is what the running Nginx would do in production.
	if err := os.WriteFile(appsettings.NginxSettings.PIDPath, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatalf("failed to seed nginx pid file: %v", err)
	}

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

	syncQueryCompleted := make(chan struct{}, 1)
	if err := db.Callback().Query().After("gorm:query").Register("test:stream_sync_query_completed", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "streams" {
			select {
			case syncQueryCompleted <- struct{}{}:
			default:
			}
		}
	}); err != nil {
		t.Fatalf("failed to register stream sync query callback: %v", err)
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
			t.Fatal("timed out waiting for stream sync query")
		}
	}

	return confDir, waitForSyncQuery
}

func TestSaveAllowsManagedStreamName(t *testing.T) {
	confDir, waitForSyncQuery := setupStreamMutationTest(t)

	err := Save("tcp_proxy", "server {\n    listen 8080;\n}\n", true, nil, "")
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	waitForSyncQuery()

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
	confDir, waitForSyncQuery := setupStreamMutationTest(t)

	if err := os.WriteFile(filepath.Join(confDir, "streams-available", "tcp_proxy"), []byte("server {\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed stream config: %v", err)
	}

	err := Rename("tcp_proxy", "tcp_proxy_new")
	if err != nil {
		t.Fatalf("Rename returned error: %v", err)
	}
	waitForSyncQuery()

	if _, err := os.Stat(filepath.Join(confDir, "streams-available", "tcp_proxy_new")); err != nil {
		t.Fatalf("expected renamed stream file: %v", err)
	}
}

func TestRenameRejectsDangerousStreamExtension(t *testing.T) {
	confDir, _ := setupStreamMutationTest(t)

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
	confDir, _ := setupStreamMutationTest(t)

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
	confDir, _ := setupStreamMutationTest(t)

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
