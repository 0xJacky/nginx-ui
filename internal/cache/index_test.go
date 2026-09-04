package cache

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useNginxHostSettings sets the nginx host mode and access mode for the
// duration of a test and restores the previous values afterwards.
func useNginxHostSettings(t *testing.T, hostMode, accessMode string) {
	t.Helper()

	originalHostMode := settings.NginxSettings.HostMode
	originalAccessMode := settings.NginxSettings.HostAccessMode
	settings.NginxSettings.HostMode = hostMode
	settings.NginxSettings.HostAccessMode = accessMode
	t.Cleanup(func() {
		settings.NginxSettings.HostMode = originalHostMode
		settings.NginxSettings.HostAccessMode = originalAccessMode
	})
}

func TestShouldPollForChanges(t *testing.T) {
	tests := []struct {
		name       string
		hostMode   string
		accessMode string
		want       bool
	}{
		{name: "local filesystem keeps fsnotify", hostMode: "", accessMode: "", want: false},
		{name: "host via ssh with sftp polls", hostMode: settings.HostModeSSH, accessMode: settings.HostAccessModeSFTP, want: true},
		{name: "host via ssh with mounted config keeps fsnotify", hostMode: settings.HostModeSSH, accessMode: settings.HostAccessModeMounted, want: false},
		{name: "invalid access mode falls back to fsnotify", hostMode: settings.HostModeSSH, accessMode: "bogus", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useNginxHostSettings(t, tt.hostMode, tt.accessMode)
			assert.Equal(t, tt.want, shouldPollForChanges())
		})
	}
}

func TestPeriodicScanIntervalUsesRemoteIntervalWhenPolling(t *testing.T) {
	originalConfig := scanConfig
	t.Cleanup(func() { scanConfig = originalConfig })
	scanConfig.PeriodicScanInterval = 5 * time.Minute
	scanConfig.RemoteScanInterval = 30 * time.Second

	local := &Scanner{}
	assert.Equal(t, 5*time.Minute, local.periodicScanInterval())

	remote := &Scanner{polling: true}
	assert.Equal(t, 30*time.Second, remote.periodicScanInterval())

	// A zero remote interval must not stall the ticker.
	scanConfig.RemoteScanInterval = 0
	assert.Equal(t, 5*time.Minute, remote.periodicScanInterval())
}

func TestScanDirectoryRecursiveReadsConfigsThroughTargetFilesystem(t *testing.T) {
	realConfDir := t.TempDir()
	confDir := filepath.Join(t.TempDir(), "nginx")
	require.NoError(t, os.Symlink(realConfDir, confDir))
	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() { settings.NginxSettings.ConfigDir = originalConfigDir })
	useNginxHostSettings(t, "", "")

	// The excluded-directory list is computed once per process; make sure it
	// is built from this test's config directory.
	excludedDirsOnce = sync.Once{}
	t.Cleanup(func() { excludedDirsOnce = sync.Once{} })

	sitesDir := filepath.Join(confDir, "sites-available")
	require.NoError(t, os.MkdirAll(sitesDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(confDir, "ssl", "example"), 0o755))

	siteConf := filepath.Join(sitesDir, "example.conf")
	require.NoError(t, os.WriteFile(siteConf, []byte("server { listen 80; }"), 0o644))
	enabledDir := filepath.Join(confDir, "sites-enabled")
	require.NoError(t, os.MkdirAll(enabledDir, 0o755))
	enabledConf := filepath.Join(enabledDir, "example.conf")
	require.NoError(t, os.Symlink(filepath.Join("..", "sites-available", "example.conf"), enabledConf))
	require.NoError(t, os.Symlink(filepath.Join(realConfDir, "sites-available", "example.conf"), filepath.Join(enabledDir, "absolute.conf")))
	require.NoError(t, os.Symlink("missing.conf", filepath.Join(enabledDir, "broken.conf")))
	mainConf := filepath.Join(confDir, "nginx.conf")
	require.NoError(t, os.WriteFile(mainConf, []byte("events {}"), 0o644))
	// Certificates live under ssl/ which the scanner must skip.
	require.NoError(t, os.WriteFile(filepath.Join(confDir, "ssl", "example", "fullchain.cer"), []byte("cert"), 0o644))
	// Non-config files outside the well-known config directories are ignored.
	require.NoError(t, os.WriteFile(filepath.Join(confDir, "README.md"), []byte("ignored"), 0o644))

	scanCallbacksMutex.Lock()
	originalCallbacks := scanCallbacks
	scanCallbacks = nil
	scanCallbacksMutex.Unlock()
	t.Cleanup(func() {
		scanCallbacksMutex.Lock()
		scanCallbacks = originalCallbacks
		scanCallbacksMutex.Unlock()
	})

	var mu sync.Mutex
	seen := map[string]string{}
	RegisterCallback("index_test", func(configPath string, content []byte) error {
		mu.Lock()
		defer mu.Unlock()
		seen[configPath] = string(content)
		return nil
	})
	indexer := newTestSearchIndexer(t, 1024)
	RegisterCallback("search_test", indexer.handleConfigScan)

	s := &Scanner{debouncer: newFileEventDebouncer()}
	fileCount, dirCount := 0, 0
	require.NoError(t, s.scanDirectoryRecursive(context.Background(), confDir, &fileCount, &dirCount))

	mu.Lock()
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	assert.Equal(t, []string{mainConf, siteConf}, paths)
	assert.Equal(t, "server { listen 80; }", seen[siteConf])
	assert.Equal(t, "events {}", seen[mainConf])
	mu.Unlock()

	// A save only rescans the available path; no alias may retain old content.
	require.NoError(t, os.WriteFile(siteConf, []byte("server { listen 9090; }"), 0o644))
	require.NoError(t, s.scanSingleFileInternal(siteConf, true))
	results, err := indexer.Search(context.Background(), "80", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
	results, err = indexer.Search(context.Background(), "9090", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, siteConf, results[0].Document.Path)

	// Removing the enabled link must leave the available config indexed.
	require.NoError(t, os.Remove(enabledConf))
	s.handleFileEvent(fsnotify.Event{Name: enabledConf, Op: fsnotify.Remove})
	results, err = indexer.Search(context.Background(), "9090", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)

	require.NoError(t, os.Remove(siteConf))
	s.handleFileEvent(fsnotify.Event{Name: siteConf, Op: fsnotify.Remove})
	results, err = indexer.Search(context.Background(), "9090", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}
