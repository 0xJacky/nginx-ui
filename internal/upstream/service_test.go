package upstream

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanForProxyTargets_IgnoresCrossFileUpstreamReferences(t *testing.T) {
	service := GetUpstreamService()
	service.ClearTargets()

	t.Cleanup(func() {
		service.ClearTargets()
	})

	siteConfig := `
server {
    listen 80;
    location / {
        proxy_pass https://my_upstream;
    }
}`

	upstreamConfig := `
upstream my_upstream {
    server my_server:8080;
}`

	if err := scanForProxyTargets("site.conf", []byte(siteConfig)); err != nil {
		t.Fatalf("scan site config failed: %v", err)
	}

	if err := scanForProxyTargets("upstream.conf", []byte(upstreamConfig)); err != nil {
		t.Fatalf("scan upstream config failed: %v", err)
	}

	targets := service.GetTargets()
	if len(targets) != 1 {
		t.Fatalf("expected 1 target after resolving cross-file upstream reference, got %d: %+v", len(targets), targets)
	}

	target := targets[0]
	if target.Host != "my_server" || target.Port != "8080" || target.Type != "upstream" || target.Scheme != "https" {
		t.Fatalf("unexpected target: %+v", target)
	}
}

func TestScanForProxyTargets_ReplacesStaleUpstreamsFromSameConfig(t *testing.T) {
	service := GetUpstreamService()
	service.ClearTargets()

	t.Cleanup(func() {
		service.ClearTargets()
	})

	initialConfig := `
upstream old_backend {
    server 127.0.0.1:8080;
}`

	updatedConfig := `
upstream new_backend {
    server 127.0.0.1:9090;
}`

	if err := scanForProxyTargets("upstream.conf", []byte(initialConfig)); err != nil {
		t.Fatalf("scan initial config failed: %v", err)
	}

	if !service.IsUpstreamName("old_backend") {
		t.Fatalf("expected old_backend to be registered")
	}

	if err := scanForProxyTargets("upstream.conf", []byte(updatedConfig)); err != nil {
		t.Fatalf("scan updated config failed: %v", err)
	}

	if service.IsUpstreamName("old_backend") {
		t.Fatalf("expected old_backend to be removed after config update")
	}

	if !service.IsUpstreamName("new_backend") {
		t.Fatalf("expected new_backend to be registered after config update")
	}
}

func TestScanForProxyTargets_ReplacesStaleTargetFromSymlinkedConfig(t *testing.T) {
	service := GetUpstreamService()
	service.ClearTargets()

	t.Cleanup(func() {
		service.ClearTargets()
	})

	root := t.TempDir()
	availableDir := filepath.Join(root, "sites-available")
	enabledDir := filepath.Join(root, "sites-enabled")
	if err := os.Mkdir(availableDir, 0o755); err != nil {
		t.Fatalf("create sites-available directory: %v", err)
	}
	if err := os.Mkdir(enabledDir, 0o755); err != nil {
		t.Fatalf("create sites-enabled directory: %v", err)
	}

	availablePath := filepath.Join(availableDir, "example")
	enabledPath := filepath.Join(enabledDir, "example")
	if err := os.WriteFile(availablePath, []byte("initial"), 0o644); err != nil {
		t.Fatalf("create available config: %v", err)
	}
	if err := os.Symlink(availablePath, enabledPath); err != nil {
		t.Fatalf("enable config: %v", err)
	}

	initialConfig := `
server {
    location / {
        proxy_pass http://192.168.10.100:8080;
    }
}`

	updatedConfig := `
server {
    location / {
        proxy_pass http://192.168.10.101:8080;
    }
}`

	if err := scanForProxyTargets(availablePath, []byte(initialConfig)); err != nil {
		t.Fatalf("scan available config: %v", err)
	}
	if err := scanForProxyTargets(enabledPath, []byte(initialConfig)); err != nil {
		t.Fatalf("scan enabled config: %v", err)
	}
	if err := scanForProxyTargets(availablePath, []byte(updatedConfig)); err != nil {
		t.Fatalf("scan updated config: %v", err)
	}

	targets := service.GetTargets()
	if len(targets) != 1 {
		t.Fatalf("expected only the updated target, got %d: %+v", len(targets), targets)
	}
	if targets[0].Host != "192.168.10.101" || targets[0].Port != "8080" {
		t.Fatalf("unexpected target after config update: %+v", targets[0])
	}

	if err := os.Remove(enabledPath); err != nil {
		t.Fatalf("disable config: %v", err)
	}
	if err := scanForProxyTargets(enabledPath, nil); err != nil {
		t.Fatalf("remove enabled config: %v", err)
	}
	if targets := service.GetTargets(); len(targets) != 1 {
		t.Fatalf("expected available config target to remain, got %d: %+v", len(targets), targets)
	}

	if err := os.Remove(availablePath); err != nil {
		t.Fatalf("remove available config: %v", err)
	}
	if err := scanForProxyTargets(availablePath, nil); err != nil {
		t.Fatalf("remove available config from scan: %v", err)
	}
	if targets := service.GetTargets(); len(targets) != 0 {
		t.Fatalf("expected target removal with the last config path, got %d: %+v", len(targets), targets)
	}
}
