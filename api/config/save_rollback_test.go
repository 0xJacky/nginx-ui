package config

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
)

// nginxTestFailureOutput is emitted by the stubbed `nginx -t` so the assertions
// can prove the reported error carries the output of the failed test.
const nginxTestFailureOutput = "nginx: [emerg] invalid directive in test"

// failingTestConfigCmd makes `nginx -t` reject the configuration on disk.
const failingTestConfigCmd = "echo '" + nginxTestFailureOutput + "' >&2; exit 1"

func assertNginxTestFailureResponse(t *testing.T, body string) {
	t.Helper()

	if !strings.Contains(body, "nginx: [emerg] invalid directive") {
		t.Fatalf("expected nginx test output in response, got %s", body)
	}
}

func TestAddConfigRemovesCreatedFileWhenNginxTestFails(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()
	appsettings.NginxSettings.TestConfigCmd = failingTestConfigCmd

	recorder := performJSONRequest(t, router, http.MethodPost, "/configs", gin.H{
		"name":    "broken.conf",
		"content": "server {\n    bogus_directive;\n}\n",
	}, map[string]string{
		"Authorization": auth.plainToken,
	})

	if recorder.Code == http.StatusOK {
		t.Fatal("expected an error response when nginx test fails")
	}
	assertNginxTestFailureResponse(t, recorder.Body.String())

	if _, err := os.Stat(filepath.Join(confDir, "broken.conf")); !os.IsNotExist(err) {
		t.Fatalf("expected the created config file to be removed, got %v", err)
	}
}

func TestAddConfigRestoresPreviousContentWhenNginxTestFails(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()

	path := filepath.Join(confDir, "app.conf")
	previousContent := "server {\n    listen 80;\n}\n"
	if err := os.WriteFile(path, []byte(previousContent), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}
	appsettings.NginxSettings.TestConfigCmd = failingTestConfigCmd

	recorder := performJSONRequest(t, router, http.MethodPost, "/configs", gin.H{
		"name":      "app.conf",
		"content":   "server {\n    bogus_directive;\n}\n",
		"overwrite": true,
	}, map[string]string{
		"Authorization": auth.plainToken,
	})

	if recorder.Code == http.StatusOK {
		t.Fatal("expected an error response when nginx test fails")
	}
	assertNginxTestFailureResponse(t, recorder.Body.String())

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if string(content) != previousContent {
		t.Fatalf("expected previous content on disk, got %q", string(content))
	}
}

func TestEditConfigKeepsPreviousContentWhenNginxTestFails(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()

	path := filepath.Join(confDir, "nginx.conf")
	previousContent := "events {}\n"
	if err := os.WriteFile(path, []byte(previousContent), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}

	reloadMarker := filepath.Join(t.TempDir(), "reload-attempted")
	appsettings.NginxSettings.TestConfigCmd = failingTestConfigCmd
	appsettings.NginxSettings.ReloadCmd = "touch " + reloadMarker

	recorder := performJSONRequest(t, router, http.MethodPost, "/config", gin.H{
		"path":    "nginx.conf",
		"content": "events {\n    bogus_directive;\n}\n",
	}, map[string]string{
		"Authorization": auth.plainToken,
	})

	if recorder.Code == http.StatusOK {
		t.Fatal("expected an error response when nginx test fails")
	}
	assertNginxTestFailureResponse(t, recorder.Body.String())

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if string(content) != previousContent {
		t.Fatalf("expected previous content on disk, got %q", string(content))
	}

	if _, err := os.Stat(reloadMarker); err == nil {
		t.Fatal("expected nginx reload to be skipped when the test fails")
	}
}

func TestSyncConfigBatchRollsBackEveryFileWhenNginxTestFails(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()

	existingPath := filepath.Join(confDir, "conf.d", "existing.conf")
	if err := os.MkdirAll(filepath.Dir(existingPath), 0o755); err != nil {
		t.Fatalf("failed to create conf.d: %v", err)
	}
	previousContent := "server {\n    listen 80;\n}\n"
	if err := os.WriteFile(existingPath, []byte(previousContent), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}
	appsettings.NginxSettings.TestConfigCmd = failingTestConfigCmd

	recorder := performJSONRequest(t, router, http.MethodPost, "/config_sync_batch", gin.H{
		"overwrite": true,
		"files": []gin.H{
			{"base_dir": "conf.d", "name": "existing.conf", "content": "server {\n    bogus_directive;\n}\n"},
			{"base_dir": "conf.d", "name": "created.conf", "content": "server {\n    listen 81;\n}\n"},
		},
	}, map[string]string{
		"Authorization": auth.plainToken,
	})

	if recorder.Code == http.StatusOK {
		t.Fatal("expected an error response when nginx test fails")
	}
	assertNginxTestFailureResponse(t, recorder.Body.String())

	content, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if string(content) != previousContent {
		t.Fatalf("expected previous content on disk, got %q", string(content))
	}

	createdPath := filepath.Join(confDir, "conf.d", "created.conf")
	if _, err := os.Stat(createdPath); !os.IsNotExist(err) {
		t.Fatalf("expected the created config file to be removed, got %v", err)
	}
}

func TestSyncConfigBatchAppliesFilesWhenNginxAcceptsThem(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()

	recorder := performJSONRequest(t, router, http.MethodPost, "/config_sync_batch", gin.H{
		"overwrite": true,
		"files": []gin.H{
			{"base_dir": "conf.d", "name": "app.conf", "content": "server {\n    listen 80;\n}\n"},
		},
	}, map[string]string{
		"Authorization": auth.plainToken,
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	content, err := os.ReadFile(filepath.Join(confDir, "conf.d", "app.conf"))
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if string(content) != "server {\n    listen 80;\n}\n" {
		t.Fatalf("expected synced content, got %q", string(content))
	}
}
