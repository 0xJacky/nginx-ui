package stream

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	appsettings "github.com/0xJacky/Nginx-UI/settings"
)

// nginxTestFailureOutput is emitted by the stubbed `nginx -t` so the assertions
// can prove the reported error carries the output of the failed test.
const nginxTestFailureOutput = "nginx: [emerg] invalid directive in test"

// failingTestConfigCmd makes `nginx -t` reject the configuration on disk.
const failingTestConfigCmd = "echo '" + nginxTestFailureOutput + "' >&2; exit 1"

func TestSaveRestoresEnabledStreamWhenTestFails(t *testing.T) {
	confDir, _ := setupStreamMutationTest(t)

	availablePath := filepath.Join(confDir, "streams-available", "tcp_proxy")
	enabledPath := filepath.Join(confDir, "streams-enabled", "tcp_proxy")
	previousContent := "server {\n    listen 8080;\n}\n"
	if err := os.WriteFile(availablePath, []byte(previousContent), 0o640); err != nil {
		t.Fatalf("failed to seed stream config: %v", err)
	}
	if err := os.Symlink(availablePath, enabledPath); err != nil {
		t.Fatalf("failed to enable stream config: %v", err)
	}
	appsettings.NginxSettings.TestConfigCmd = failingTestConfigCmd

	err := Save("tcp_proxy", "server {\n    bogus_directive;\n}\n", true, nil, "")

	if err == nil {
		t.Fatal("Save expected an error when nginx test fails")
	}
	if !strings.Contains(err.Error(), nginxTestFailureOutput) {
		t.Fatalf("expected nginx test output in error, got %v", err)
	}

	content, readErr := os.ReadFile(availablePath)
	if readErr != nil {
		t.Fatalf("failed to read stream config: %v", readErr)
	}
	if string(content) != previousContent {
		t.Fatalf("expected previous content on disk, got %q", string(content))
	}

	info, statErr := os.Stat(availablePath)
	if statErr != nil {
		t.Fatalf("failed to stat stream config: %v", statErr)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("expected restored permissions 0640, got %v", info.Mode().Perm())
	}
}
