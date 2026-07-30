package clustersync

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

// withConfDir points the Nginx configuration root at a temporary directory
// populated with the given relative files.
func withConfDir(t *testing.T, files map[string]string) string {
	t.Helper()

	originalConfigDir := settings.NginxSettings.ConfigDir
	originalConfigPath := settings.NginxSettings.ConfigPath
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		settings.NginxSettings.ConfigPath = originalConfigPath
	})

	confDir := t.TempDir()
	settings.NginxSettings.ConfigDir = confDir
	settings.NginxSettings.ConfigPath = filepath.Join(confDir, "nginx.conf")

	for relativePath, content := range files {
		path := filepath.Join(confDir, relativePath)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	return confDir
}

func collectedPaths(files []ConfigFile) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.RelativePath())
	}
	sort.Strings(paths)

	return paths
}

func TestCollectConfigFilesWalksNestedDirectories(t *testing.T) {
	confDir := withConfDir(t, map[string]string{
		"nginx.conf":            "events {}\n",
		"conf.d/a.conf":         "# a\n",
		"conf.d/nested/b.conf":  "# b\n",
		"snippets/tls.conf":     "# tls\n",
		"sites-available/site1": "server {}\n",
		"sites-enabled/site1":   "server {}\n",
		"streams-available/s1":  "server {}\n",
	})

	files, err := CollectConfigFiles(filepath.Join(confDir, "conf.d"))
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	got := collectedPaths(files)
	want := []string{"conf.d/a.conf", "conf.d/nested/b.conf"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestCollectConfigFilesSkipsSitesAndStreams(t *testing.T) {
	confDir := withConfDir(t, map[string]string{
		"nginx.conf":            "events {}\n",
		"conf.d/a.conf":         "# a\n",
		"sites-available/site1": "server {}\n",
		"sites-enabled/site1":   "server {}\n",
		"streams-available/s1":  "server {}\n",
		"streams-enabled/s1":    "server {}\n",
	})

	files, err := CollectConfigFiles(confDir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	for _, path := range collectedPaths(files) {
		if path == "sites-available/site1" || path == "streams-available/s1" ||
			path == "sites-enabled/site1" || path == "streams-enabled/s1" {
			t.Fatalf("sites and streams must be replicated as sites and streams, got %s", path)
		}
	}

	got := collectedPaths(files)
	if len(got) != 1 || got[0] != "conf.d/a.conf" {
		t.Fatalf("expected only conf.d/a.conf, got %v", got)
	}
}

func TestCollectConfigFilesSkipsTheEntryConfiguration(t *testing.T) {
	confDir := withConfDir(t, map[string]string{
		"nginx.conf":    "events {}\n",
		"conf.d/a.conf": "# a\n",
	})

	files, err := CollectConfigFiles(confDir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	for _, path := range collectedPaths(files) {
		if path == "nginx.conf" {
			t.Fatal("the node specific entry configuration must not be replicated in bulk")
		}
	}
}

func TestCollectConfigFilesSkipsBinaryAndOversizedFiles(t *testing.T) {
	confDir := withConfDir(t, map[string]string{
		"nginx.conf": "events {}\n",
	})

	if err := os.WriteFile(filepath.Join(confDir, "geoip.mmdb"), []byte{0xff, 0xfe, 0x00, 0x01}, 0644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "huge.conf"), make([]byte, maxSyncFileSize+1), 0644); err != nil {
		t.Fatalf("write oversized: %v", err)
	}

	files, err := CollectConfigFiles(confDir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	if got := collectedPaths(files); len(got) != 0 {
		t.Fatalf("expected binary and oversized files to be skipped, got %v", got)
	}
}

func TestCollectConfigFilesRejectsPathOutsideConfDir(t *testing.T) {
	withConfDir(t, map[string]string{"nginx.conf": "events {}\n"})

	if _, err := CollectConfigFiles(t.TempDir()); err == nil {
		t.Fatal("expected an error for a path outside the nginx conf dir")
	}
}

func TestConfigFileRelativePathHandlesRootFiles(t *testing.T) {
	file := ConfigFile{BaseDir: "", Name: "nginx.conf"}
	if got := file.RelativePath(); got != "nginx.conf" {
		t.Fatalf("got %q, want %q", got, "nginx.conf")
	}

	file = ConfigFile{BaseDir: "conf.d", Name: "a.conf"}
	if got := file.RelativePath(); got != "conf.d/a.conf" {
		t.Fatalf("got %q, want %q", got, "conf.d/a.conf")
	}
}
