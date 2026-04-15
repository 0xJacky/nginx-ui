package upstream

import "testing"

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
        proxy_pass http://my_upstream;
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
	if target.Host != "my_server" || target.Port != "8080" || target.Type != "upstream" {
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
