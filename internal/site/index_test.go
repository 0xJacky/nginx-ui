package site

import (
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestScanForSiteHandlesWildcardAndProxyProtocol(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	tmpDir := t.TempDir()
	settings.NginxSettings.ConfigDir = tmpDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		siteIndexMutex.Lock()
		delete(IndexedSites, "wildcard.conf")
		siteIndexMutex.Unlock()
	})

	configPath := filepath.Join(tmpDir, "wildcard.conf")
	config := []byte(`
server {
    listen 8443 ssl proxy_protocol;
    server_name *.example.com;
}
`)

	if err := scanForSite(configPath, config); err != nil {
		t.Fatalf("scanForSite returned error: %v", err)
	}

	siteIndexMutex.RLock()
	indexed := IndexedSites["wildcard.conf"]
	siteIndexMutex.RUnlock()

	if indexed == nil {
		t.Fatal("expected indexed site to be populated")
	}

	if got := len(indexed.Urls); got != 1 {
		t.Fatalf("expected 1 URL, got %d", got)
	}

	rawURL := "https://example.com:8443"
	if indexed.Urls[0] != rawURL {
		t.Fatalf("expected raw URL %s, got %s", rawURL, indexed.Urls[0])
	}

	displayURL := indexed.GetDisplayURL(rawURL)
	if displayURL != "https://example.com" {
		t.Fatalf("expected display URL %s, got %s", "https://example.com", displayURL)
	}
}
