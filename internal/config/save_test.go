package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// nginxTestFailureOutput is emitted by the stubbed `nginx -t` so the assertions
// can prove the reported error carries the output of the failed test.
const nginxTestFailureOutput = "nginx: [emerg] invalid directive in test"

// failingTestConfigCmd makes `nginx -t` reject the configuration on disk.
const failingTestConfigCmd = "echo '" + nginxTestFailureOutput + "' >&2; exit 1"

// setupConfigSaveTest prepares an isolated Nginx configuration directory plus an
// in-memory database so Save can be exercised end to end.
func setupConfigSaveTest(t *testing.T) string {
	t.Helper()

	confDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(confDir, "conf.d"), 0o755); err != nil {
		t.Fatalf("failed to create conf.d: %v", err)
	}

	originalNginxSettings := *settings.NginxSettings
	settings.NginxSettings.ConfigDir = confDir
	settings.NginxSettings.PIDPath = filepath.Join(confDir, "nginx.pid")
	settings.NginxSettings.ReloadCmd = "true"
	settings.NginxSettings.RestartCmd = "true"
	settings.NginxSettings.TestConfigCmd = "true"
	if err := os.WriteFile(settings.NginxSettings.PIDPath, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatalf("failed to seed nginx pid file: %v", err)
	}

	t.Cleanup(func() {
		*settings.NginxSettings = originalNginxSettings
	})

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.Config{}, &model.ConfigBackup{}, &model.Node{}, &model.LLMSession{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	return confDir
}

func TestSaveKeepsPreviousContentWhenNginxTestFails(t *testing.T) {
	confDir := setupConfigSaveTest(t)

	absPath := filepath.Join(confDir, "conf.d", "app.conf")
	previousContent := "server {\n    listen 80;\n}\n"
	if err := os.WriteFile(absPath, []byte(previousContent), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}

	reloadMarker := filepath.Join(t.TempDir(), "reload-attempted")
	settings.NginxSettings.TestConfigCmd = failingTestConfigCmd
	settings.NginxSettings.ReloadCmd = fmt.Sprintf("touch %q", reloadMarker)

	err := Save(absPath, "server {\n    bogus_directive;\n}\n", nil)

	if err == nil {
		t.Fatal("Save expected an error when nginx test fails")
	}
	if !strings.Contains(err.Error(), nginxTestFailureOutput) {
		t.Fatalf("expected nginx test output in error, got %v", err)
	}

	content, readErr := os.ReadFile(absPath)
	if readErr != nil {
		t.Fatalf("failed to read config file: %v", readErr)
	}
	if string(content) != previousContent {
		t.Fatalf("expected previous content on disk, got %q", string(content))
	}

	if _, statErr := os.Stat(reloadMarker); statErr == nil {
		t.Fatal("expected nginx reload to be skipped when the test fails")
	}
}

func TestSaveRemovesCreatedFileWhenNginxTestFails(t *testing.T) {
	confDir := setupConfigSaveTest(t)

	absPath := filepath.Join(confDir, "conf.d", "created.conf")
	settings.NginxSettings.TestConfigCmd = failingTestConfigCmd

	err := Save(absPath, "server {\n    bogus_directive;\n}\n", nil)

	if err == nil {
		t.Fatal("Save expected an error when nginx test fails")
	}
	if _, statErr := os.Stat(absPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected the created file to be removed, got %v", statErr)
	}
}

func TestSaveRestoresPreviousContentWhenReloadFails(t *testing.T) {
	confDir := setupConfigSaveTest(t)

	absPath := filepath.Join(confDir, "conf.d", "app.conf")
	previousContent := "server {\n    listen 80;\n}\n"
	if err := os.WriteFile(absPath, []byte(previousContent), 0o640); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}

	// Fail the first reload only, so the rollback can load the restored file.
	reloadMarker := filepath.Join(t.TempDir(), "reload-attempted")
	settings.NginxSettings.ReloadCmd = fmt.Sprintf(
		"if [ ! -e %q ]; then touch %q; exit 1; fi", reloadMarker, reloadMarker)

	err := Save(absPath, "server {\n    listen 81;\n}\n", nil)

	if err == nil {
		t.Fatal("Save expected an error when nginx reload fails")
	}

	content, readErr := os.ReadFile(absPath)
	if readErr != nil {
		t.Fatalf("failed to read config file: %v", readErr)
	}
	if string(content) != previousContent {
		t.Fatalf("expected previous content on disk, got %q", string(content))
	}

	info, statErr := os.Stat(absPath)
	if statErr != nil {
		t.Fatalf("failed to stat config file: %v", statErr)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("expected restored permissions 0640, got %v", info.Mode().Perm())
	}
}

func TestSaveWritesContentWhenNginxAcceptsIt(t *testing.T) {
	confDir := setupConfigSaveTest(t)

	absPath := filepath.Join(confDir, "conf.d", "app.conf")
	newContent := "server {\n    listen 8080;\n}\n"

	if err := Save(absPath, newContent, nil); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	content, readErr := os.ReadFile(absPath)
	if readErr != nil {
		t.Fatalf("failed to read config file: %v", readErr)
	}
	if string(content) != newContent {
		t.Fatalf("expected saved content, got %q", string(content))
	}
}
