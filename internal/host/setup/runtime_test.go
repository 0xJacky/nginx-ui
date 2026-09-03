package setup

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestNewClientFromParamsUsesContainerKeyPath(t *testing.T) {
	originalKnownHostsPath := settings.NginxSettings.HostKnownHostsPath
	settings.NginxSettings.HostKnownHostsPath = filepath.Join(t.TempDir(), "known_hosts")
	t.Cleanup(func() {
		settings.NginxSettings.HostKnownHostsPath = originalKnownHostsPath
	})

	privateKeyPath := filepath.Join(t.TempDir(), "selected_key")
	client, err := NewClientFromParams(SetupParams{
		HostAddress:      "127.0.0.1:1",
		HostUser:         "nginxui",
		ContainerKeyPath: privateKeyPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	_, err = client.Exec(context.Background(), "/bin/echo", "test")
	if err == nil || !strings.Contains(err.Error(), privateKeyPath) {
		t.Fatalf("authentication error does not reference selected key path: %v", err)
	}
}
