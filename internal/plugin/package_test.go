package plugin

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestExtractPackageOnlyMarksManifestExecutables(t *testing.T) {
	source := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(source, "bin"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(source, "webapp"), 0o755))
	// Everything is packed 0755, as a Windows mount or some CI images do.
	for rel, body := range map[string]string{
		ManifestFileName: manifestJSON(t),
		"bin/plugin":     "#!/bin/sh\n",
		"webapp/main.js": "export default {}",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(source, filepath.FromSlash(rel)), []byte(body), 0o755))
	}

	archivePath := filepath.Join(t.TempDir(), "plugin.tar.gz")
	require.NoError(t, BuildPackage(source, archivePath))

	dest := filepath.Join(t.TempDir(), "installed")
	_, err := ExtractPackage(archivePath, dest)
	require.NoError(t, err)

	for rel, want := range map[string]os.FileMode{
		ManifestFileName: 0o644,
		"webapp/main.js": 0o644,
		"bin/plugin":     0o755,
	} {
		info, err := os.Stat(filepath.Join(dest, filepath.FromSlash(rel)))
		require.NoError(t, err)
		assert.Equal(t, want, info.Mode().Perm(), rel)
	}
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
		{
			name: "undecodable manifest",
			entries: []tarEntry{
				{header: tar.Header{Name: ManifestFileName, Typeflag: tar.TypeReg}, body: `{not json`},
			},
			target: ErrManifestInvalid,
		},
		{
			name: "executable is a directory",
			entries: []tarEntry{
				{header: tar.Header{Name: ManifestFileName, Typeflag: tar.TypeReg}, body: manifest},
				{header: tar.Header{Name: "bin/plugin/", Typeflag: tar.TypeDir, Mode: 0o755}},
			},
			target: ErrPackageInvalid,
		},
		{
			name: "executable is an implied directory",
			entries: []tarEntry{
				{header: tar.Header{Name: ManifestFileName, Typeflag: tar.TypeReg}, body: manifest},
				{header: tar.Header{Name: "bin/plugin/inner", Typeflag: tar.TypeReg}, body: "x"},
			},
			target: ErrPackageInvalid,
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

			// Peeking refuses the same package with the same error.
			_, _, err = peekPackage(archivePath)
			assert.ErrorIs(t, err, tc.target)
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

	_, _, err = peekPackage(archivePath)
	assert.ErrorIs(t, err, ErrPackageTooLarge)
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

	_, _, err = peekPackage(archivePath)
	assert.ErrorIs(t, err, ErrPackageTooLarge)
}

func TestExtractPackageRejectsBrokenArchive(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "package.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, []byte("this is not a gzip stream"), 0o644))

	_, err := ExtractPackage(archivePath, filepath.Join(t.TempDir(), "installed"))
	assert.ErrorIs(t, err, ErrPackageInvalid)
	_, _, err = peekPackage(archivePath)
	assert.ErrorIs(t, err, ErrPackageInvalid)

	_, err = ExtractPackage(filepath.Join(t.TempDir(), "missing.tar.gz"), t.TempDir())
	assert.Error(t, err)
	_, _, err = peekPackage(filepath.Join(t.TempDir(), "missing.tar.gz"))
	assert.ErrorIs(t, err, os.ErrNotExist)
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

func TestBuildSignedPackageEmbedsSumsAndSignature(t *testing.T) {
	signer := useReleaseKey(t)
	archive := signedWebappPackage(t, "com.example.alpha", signer)

	body, err := os.ReadFile(archive)
	require.NoError(t, err)
	entries := readPackageEntries(t, body)
	require.GreaterOrEqual(t, len(entries), 2)
	// The two signature files are the last entries.
	assert.Equal(t, SumsFileName, entries[len(entries)-2].header.Name)
	assert.Equal(t, SumsSignatureFileName, entries[len(entries)-1].header.Name)

	sums := entries[len(entries)-2].body
	lines := strings.Split(strings.TrimSuffix(sums, "\n"), "\n")
	names := make([]string, 0, len(lines))
	for _, line := range lines {
		digest, name, ok := strings.Cut(line, "  ")
		require.True(t, ok, line)
		assert.Len(t, digest, 64)
		names = append(names, name)
	}
	// Every regular file, sorted bytewise, directories and the signature
	// files left out.
	assert.Equal(t, []string{"README.md", ManifestFileName, "webapp/main.js"}, names)
	assert.True(t, strings.HasSuffix(sums, "\n"))

	trust, err := extractedTrust(t, archive, "")
	require.NoError(t, err)
	assert.Equal(t, TrustOfficial, trust.Trust)
	assert.Equal(t, keyID(signer), trust.Signer)
}

func TestBuildPackageDropsAStaleSignature(t *testing.T) {
	signer := useReleaseKey(t)
	signed := signedWebappPackage(t, "com.example.alpha", signer)
	source := filepath.Join(t.TempDir(), "source")
	_, err := ExtractPackage(signed, source)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(source, SumsFileName))

	// Packing the extracted copy yields an unsigned package.
	unsigned := filepath.Join(t.TempDir(), "unsigned.tar.gz")
	require.NoError(t, BuildPackage(source, unsigned))
	body, err := os.ReadFile(unsigned)
	require.NoError(t, err)
	for _, entry := range readPackageEntries(t, body) {
		assert.False(t, isSignatureFile(entry.header.Name), entry.header.Name)
	}

	// Signing it again with another key replaces the signature.
	other := usePartnerKey(t)
	resigned := filepath.Join(t.TempDir(), "resigned.tar.gz")
	require.NoError(t, BuildSignedPackage(source, resigned, *other))
	trust, err := extractedTrust(t, resigned, "")
	require.NoError(t, err)
	assert.Equal(t, TrustVerified, trust.Trust)
	assert.Equal(t, keyID(other), trust.Signer)
}

func TestSignPackageSignsInPlace(t *testing.T) {
	signer := useReleaseKey(t)
	archive := signedWebappPackage(t, "com.example.alpha", nil)
	require.NoError(t, os.Chmod(archive, 0o640))

	trust, err := extractedTrust(t, archive, "")
	require.NoError(t, err)
	assert.Equal(t, TrustUnsigned, trust.Trust)

	require.NoError(t, SignPackage(archive, *signer))
	trust, err = extractedTrust(t, archive, "")
	require.NoError(t, err)
	assert.Equal(t, TrustOfficial, trust.Trust)
	assert.Equal(t, keyID(signer), trust.Signer)

	info, err := os.Stat(archive)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
	// No temporary file is left next to the package.
	items, err := os.ReadDir(filepath.Dir(archive))
	require.NoError(t, err)
	assert.Len(t, items, 1)

	// A package that is not one is refused and left alone.
	broken := filepath.Join(t.TempDir(), "broken.tar.gz")
	require.NoError(t, os.WriteFile(broken, []byte("not a package"), 0o644))
	assert.ErrorIs(t, SignPackage(broken, *signer), ErrPackageInvalid)
}
