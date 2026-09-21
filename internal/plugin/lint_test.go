package plugin

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lintFixture describes one plugin directory to write for a lint test.
type lintFixture struct {
	manifest       *protocol.Manifest
	skipReadme     bool
	skipLicense    bool
	skipChangelog  bool
	skipExecutable bool
}

// goodManifest is a minimal, fully valid manifest every negative test starts
// from and mutates one field of.
func goodManifest() *protocol.Manifest {
	platform := runtime.GOOS + "-" + runtime.GOARCH
	return &protocol.Manifest{
		ID:         "io.github.example.mydns",
		Name:       "MyDNS",
		Version:    "0.1.0",
		APIVersion: protocol.APIVersion,
		Server: &protocol.ManifestServer{
			Executables: map[string]string{platform: "bin/plugin"},
		},
		Capabilities: []string{protocol.CapabilityDNS01},
		Permissions:  []string{protocol.PermissionNetwork},
		DNS01: &protocol.ManifestDNS01{
			Providers: []protocol.DNS01Provider{
				{
					Name: "MyDNS",
					Code: "mydns",
					Configuration: &protocol.DNS01ProviderConfig{
						Credentials: map[string]string{"MYDNS_API_TOKEN": "API token"},
					},
				},
			},
		},
	}
}

// writeLintFixture materialises a plugin directory for f.manifest, including
// the docs and executables the good manifest expects, unless the fixture
// asks for one of them to be skipped.
func writeLintFixture(t *testing.T, f lintFixture) string {
	t.Helper()
	dir := t.TempDir()

	data, err := json.MarshalIndent(f.manifest, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ManifestFileName), data, 0o644))

	if f.manifest.Server != nil && !f.skipExecutable {
		for _, rel := range f.manifest.Server.Executables {
			target := filepath.Join(dir, filepath.FromSlash(rel))
			require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
			require.NoError(t, os.WriteFile(target, []byte("#!/bin/sh\nexit 0\n"), 0o755))
		}
	}

	if !f.skipReadme {
		readme := "# Test Plugin\n\n## Permissions\n\nNone.\n\n## Supported platforms\n\nAll.\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644))
	}
	if !f.skipLicense {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "LICENSE"), []byte("MIT"), 0o644))
	}
	if !f.skipChangelog {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte("# Changelog\n"), 0o644))
	}
	return dir
}

// assertHasFinding fails the test unless report contains a Finding with the
// given level and rule.
func assertHasFinding(t *testing.T, report *LintReport, level Level, rule string) {
	t.Helper()
	for _, f := range report.Findings {
		if f.Level == level && f.Rule == rule {
			return
		}
	}
	t.Fatalf("expected a %s %s finding, got %+v", level, rule, report.Findings)
}

func TestLintGoodManifestHasNoFindings(t *testing.T) {
	dir := writeLintFixture(t, lintFixture{manifest: goodManifest()})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.Empty(t, report.Findings, "%+v", report.Findings)
	assert.False(t, report.HasErrors())
}

func TestLintPathDoesNotExist(t *testing.T) {
	_, err := Lint(filepath.Join(t.TempDir(), "does-not-exist"))
	assert.Error(t, err)
}

func TestLintMissingReadmeIsError(t *testing.T) {
	dir := writeLintFixture(t, lintFixture{manifest: goodManifest(), skipReadme: true})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.True(t, report.HasErrors())
	assertHasFinding(t, report, LevelError, "PKG-8")
}

func TestLintMissingLicenseAndChangelogAreWarnings(t *testing.T) {
	dir := writeLintFixture(t, lintFixture{manifest: goodManifest(), skipLicense: true, skipChangelog: true})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.False(t, report.HasErrors())
	require.Len(t, report.Findings, 2)
	for _, f := range report.Findings {
		assert.Equal(t, LevelWarning, f.Level)
		assert.Equal(t, "PKG-8", f.Rule)
	}
}

func TestLintMissingExecutableIsError(t *testing.T) {
	dir := writeLintFixture(t, lintFixture{manifest: goodManifest(), skipExecutable: true})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.True(t, report.HasErrors())
	assertHasFinding(t, report, LevelError, "PKG-9")
}

func TestLintBadProviderCodeIsError(t *testing.T) {
	m := goodManifest()
	m.DNS01.Providers[0].Code = "BAD CODE!"
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.True(t, report.HasErrors())
	assertHasFinding(t, report, LevelError, "DNS01-2")
}

func TestLintBadIDIsError(t *testing.T) {
	m := goodManifest()
	m.ID = "NotValid"
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.True(t, report.HasErrors())
	assertHasFinding(t, report, LevelError, "MAN-2")
}

func TestLintReservedNamespaceWarns(t *testing.T) {
	m := goodManifest()
	m.ID = "com.nginxui.example"
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.False(t, report.HasErrors())
	assertHasFinding(t, report, LevelWarning, "NAME-2")
}

func TestLintUnknownPermissionIsError(t *testing.T) {
	m := goodManifest()
	m.Permissions = append(m.Permissions, "not-a-real-permission")
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.True(t, report.HasErrors())
	assertHasFinding(t, report, LevelError, "MAN-23")
}

func TestLintSettingsSchemaSelectRequiresOptions(t *testing.T) {
	m := goodManifest()
	m.SettingsSchema = &protocol.SettingsSchema{
		Settings: []protocol.SettingsField{{Key: "mode", Type: "select", DisplayName: "Mode"}},
	}
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.True(t, report.HasErrors())
	assertHasFinding(t, report, LevelError, "MAN-29")
}

func TestLintArchivePackageMatchesDirectory(t *testing.T) {
	dir := writeLintFixture(t, lintFixture{manifest: goodManifest()})
	archive := filepath.Join(t.TempDir(), "pkg.tar.gz")
	require.NoError(t, BuildPackage(dir, archive))

	report, err := Lint(archive)
	require.NoError(t, err)
	assert.Empty(t, report.Findings, "%+v", report.Findings)
}

func TestLintArchiveWithSymlinkIsError(t *testing.T) {
	dir := writeLintFixture(t, lintFixture{manifest: goodManifest()})
	archive := filepath.Join(t.TempDir(), "pkg.tar.gz")
	writeArchiveWithSymlink(t, dir, archive)

	report, err := Lint(archive)
	require.NoError(t, err)
	assert.True(t, report.HasErrors())
	assertHasFinding(t, report, LevelError, "PKG-4")
}

func TestLintDirectorySymlinkIsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevated privileges on windows")
	}
	dir := writeLintFixture(t, lintFixture{manifest: goodManifest()})
	require.NoError(t, os.Symlink(filepath.Join(dir, "README.md"), filepath.Join(dir, "evil-link")))

	report, err := Lint(dir)
	require.NoError(t, err)
	assertHasFinding(t, report, LevelError, "PKG-4")
}

// writeArchiveWithSymlink packages srcDir like BuildPackage, plus one
// dangling symlink entry PKG-4 forbids regardless of its target.
func writeArchiveWithSymlink(t *testing.T, srcDir, archivePath string) {
	t.Helper()

	out, err := os.Create(archivePath)
	require.NoError(t, err)
	defer out.Close()

	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)

	err = filepath.WalkDir(srcDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return tw.WriteHeader(&tar.Header{Name: filepath.ToSlash(rel) + "/", Typeflag: tar.TypeDir, Mode: 0o755})
		}
		if err := tw.WriteHeader(&tar.Header{
			Name: filepath.ToSlash(rel), Typeflag: tar.TypeReg, Mode: int64(info.Mode().Perm()), Size: info.Size(),
		}); err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = tw.Write(content)
		return err
	})
	require.NoError(t, err)

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "evil-link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd",
	}))

	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
}
