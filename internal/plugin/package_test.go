package plugin

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tarEntry is one entry of a hand built archive.
type tarEntry struct {
	header tar.Header
	body   string
}

// writeArchive builds a .tar.gz from raw headers so the tests can produce the
// archives a well behaved writer would refuse to create.
func writeArchive(t *testing.T, entries []tarEntry) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "package.tar.gz")
	file, err := os.Create(archivePath)
	require.NoError(t, err)
	defer file.Close()

	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		header := entry.header
		if header.Typeflag == tar.TypeReg && header.Size == 0 {
			header.Size = int64(len(entry.body))
		}
		if header.Mode == 0 {
			header.Mode = 0o644
		}
		require.NoError(t, tw.WriteHeader(&header))
		if entry.body != "" {
			_, err = tw.Write([]byte(entry.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return archivePath
}

func manifestJSON(t *testing.T) string {
	t.Helper()
	m := validManifest()
	m.Server.Executables = map[string]string{"linux-amd64": "bin/plugin"}
	data, err := json.Marshal(m)
	require.NoError(t, err)
	return string(data)
}

func TestBuildAndExtractPackage(t *testing.T) {
	source := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(source, "bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(source, ManifestFileName), []byte(manifestJSON(t)), 0o644))
	// The binary is shipped without the executable bit on purpose.
	require.NoError(t, os.WriteFile(filepath.Join(source, "bin", "plugin"), []byte("#!/bin/sh\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(source, "README.md"), []byte("hello"), 0o644))

	archivePath := filepath.Join(t.TempDir(), "plugin.tar.gz")
	require.NoError(t, BuildPackage(source, archivePath))

	dest := filepath.Join(t.TempDir(), "installed")
	manifest, err := ExtractPackage(archivePath, dest)
	require.NoError(t, err)
	assert.Equal(t, "official.cloudflare", manifest.ID)

	content, err := os.ReadFile(filepath.Join(dest, "README.md"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(content))

	info, err := os.Stat(filepath.Join(dest, "bin", "plugin"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm(), "the manifest executable must be runnable")
}

func TestExtractPackageStripsWrapperDirectory(t *testing.T) {
	archivePath := writeArchive(t, []tarEntry{
		{header: tar.Header{Name: "cloudflare/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{header: tar.Header{Name: "cloudflare/" + ManifestFileName, Typeflag: tar.TypeReg}, body: manifestJSON(t)},
		{header: tar.Header{Name: "cloudflare/bin/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{header: tar.Header{Name: "cloudflare/bin/plugin", Typeflag: tar.TypeReg, Mode: 0o644}, body: "binary"},
	})

	dest := filepath.Join(t.TempDir(), "installed")
	manifest, err := ExtractPackage(archivePath, dest)
	require.NoError(t, err)
	assert.Equal(t, "official.cloudflare", manifest.ID)
	assert.FileExists(t, filepath.Join(dest, ManifestFileName))
	assert.FileExists(t, filepath.Join(dest, "bin", "plugin"))
	assert.NoDirExists(t, filepath.Join(dest, "cloudflare"))
}

func TestExtractPackageRejections(t *testing.T) {
	manifest := manifestJSON(t)
	tests := []struct {
		name    string
		entries []tarEntry
		target  error
	}{
		{
			name: "symlink",
			entries: []tarEntry{
				{header: tar.Header{Name: ManifestFileName, Typeflag: tar.TypeReg}, body: manifest},
				{header: tar.Header{Name: "evil", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}},
			},
			target: ErrPackageInvalid,
		},
		{
			name: "hard link",
			entries: []tarEntry{
				{header: tar.Header{Name: ManifestFileName, Typeflag: tar.TypeReg}, body: manifest},
				{header: tar.Header{Name: "evil", Typeflag: tar.TypeLink, Linkname: ManifestFileName}},
			},
			target: ErrPackageInvalid,
		},
		{
			name: "absolute path",
			entries: []tarEntry{
				{header: tar.Header{Name: "/etc/cron.d/evil", Typeflag: tar.TypeReg}, body: "x"},
			},
			target: ErrPackageInvalid,
		},
		{
			name: "traversal",
			entries: []tarEntry{
				{header: tar.Header{Name: "../../evil", Typeflag: tar.TypeReg}, body: "x"},
			},
			target: ErrPackageInvalid,
		},
		{
			name: "fifo",
			entries: []tarEntry{
				{header: tar.Header{Name: ManifestFileName, Typeflag: tar.TypeReg}, body: manifest},
				{header: tar.Header{Name: "pipe", Typeflag: tar.TypeFifo}},
			},
			target: ErrPackageInvalid,
		},
		{
			name: "no manifest",
			entries: []tarEntry{
				{header: tar.Header{Name: "README.md", Typeflag: tar.TypeReg}, body: "x"},
			},
			target: ErrPackageInvalid,
		},
		{
			name: "two top level directories",
			entries: []tarEntry{
				{header: tar.Header{Name: "one/" + ManifestFileName, Typeflag: tar.TypeReg}, body: manifest},
				{header: tar.Header{Name: "two/README.md", Typeflag: tar.TypeReg}, body: "x"},
			},
			target: ErrPackageInvalid,
		},
		{
			name: "invalid manifest",
			entries: []tarEntry{
				{header: tar.Header{Name: ManifestFileName, Typeflag: tar.TypeReg}, body: `{"id":"nope"}`},
			},
			target: ErrManifestInvalid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			archivePath := writeArchive(t, tc.entries)
			dest := filepath.Join(t.TempDir(), "installed")
			_, err := ExtractPackage(archivePath, dest)
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.target)
			assert.NoDirExists(t, dest, "a failed extraction must not leave anything behind")
		})
	}
}

func TestExtractPackageRefusesTooManyFiles(t *testing.T) {
	entries := make([]tarEntry, 0, MaxPackageFiles+2)
	entries = append(entries, tarEntry{
		header: tar.Header{Name: ManifestFileName, Typeflag: tar.TypeReg},
		body:   manifestJSON(t),
	})
	for i := range MaxPackageFiles + 1 {
		entries = append(entries, tarEntry{
			header: tar.Header{Name: fmt.Sprintf("files/%d.txt", i), Typeflag: tar.TypeReg},
		})
	}

	archivePath := writeArchive(t, entries)
	dest := filepath.Join(t.TempDir(), "installed")
	_, err := ExtractPackage(archivePath, dest)
	assert.ErrorIs(t, err, ErrPackageTooLarge)
	assert.NoDirExists(t, dest)
}

func TestExtractPackageRefusesOversizeEntry(t *testing.T) {
	// The header alone is enough: the size is checked before the body is read.
	archivePath := filepath.Join(t.TempDir(), "package.tar.gz")
	file, err := os.Create(archivePath)
	require.NoError(t, err)
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "huge.bin",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     MaxPackageSize + 1,
	}))
	// The body is deliberately missing, so the tar writer is not closed.
	require.NoError(t, gz.Close())
	require.NoError(t, file.Close())

	dest := filepath.Join(t.TempDir(), "installed")
	_, err = ExtractPackage(archivePath, dest)
	assert.ErrorIs(t, err, ErrPackageTooLarge)
}

func TestExtractPackageRejectsBrokenArchive(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "package.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, []byte("this is not a gzip stream"), 0o644))

	_, err := ExtractPackage(archivePath, filepath.Join(t.TempDir(), "installed"))
	assert.ErrorIs(t, err, ErrPackageInvalid)

	_, err = ExtractPackage(filepath.Join(t.TempDir(), "missing.tar.gz"), t.TempDir())
	assert.Error(t, err)
}

func TestBuildPackageSkipsLinks(t *testing.T) {
	source := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(source, ManifestFileName), []byte(manifestJSON(t)), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(source, "bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(source, "bin", "plugin"), []byte("binary"), 0o755))
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(source, "link")))

	archivePath := filepath.Join(t.TempDir(), "plugin.tar.gz")
	require.NoError(t, BuildPackage(source, archivePath))

	dest := filepath.Join(t.TempDir(), "installed")
	_, err := ExtractPackage(archivePath, dest)
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(dest, "link"))
}

func TestMarkExecutablesIgnoresMissingPlatforms(t *testing.T) {
	dir := t.TempDir()
	m := &protocol.Manifest{Server: &protocol.ManifestServer{
		Executables: map[string]string{
			"linux-amd64":   "bin/plugin-linux",
			"windows-amd64": "bin/plugin.exe",
		},
		Command: []string{"venv/bin/python", "main.py"},
	}}
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bin", "plugin-linux"), []byte("x"), 0o600))

	require.NoError(t, markExecutables(m, dir))
	info, err := os.Stat(filepath.Join(dir, "bin", "plugin-linux"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}
