package plugin

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lintFixture describes one plugin directory to write for a lint test.
type lintFixture struct {
	manifest       *protocol.Manifest
	skipReadme     bool
	skipLicense    bool
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
					Form: &protocol.DNS01ProviderForm{
						Fields: []protocol.DNS01ProviderField{{Key: "MYDNS_API_TOKEN", Label: "API token", Group: "credential", Secret: true}},
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
	assertHasFinding(t, report, LevelError, RulePackageDocs)
}

func TestLintMissingLicenseIsWarning(t *testing.T) {
	dir := writeLintFixture(t, lintFixture{manifest: goodManifest(), skipLicense: true})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.False(t, report.HasErrors())
	require.Len(t, report.Findings, 1)
	assert.Equal(t, LevelWarning, report.Findings[0].Level)
	assert.Equal(t, RulePackageDocs, report.Findings[0].Rule)
}

func TestLintMissingExecutableIsError(t *testing.T) {
	dir := writeLintFixture(t, lintFixture{manifest: goodManifest(), skipExecutable: true})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.True(t, report.HasErrors())
	assertHasFinding(t, report, LevelError, RulePackageExecutables)
}

func TestLintBadProviderCodeIsError(t *testing.T) {
	m := goodManifest()
	m.DNS01.Providers[0].Code = "BAD CODE!"
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.True(t, report.HasErrors())
	assertHasFinding(t, report, LevelError, RuleDNS01Code)
}

func TestLintBadIDIsError(t *testing.T) {
	m := goodManifest()
	m.ID = "NotValid"
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.True(t, report.HasErrors())
	assertHasFinding(t, report, LevelError, RuleManifestID)
}

func TestLintI18nLocales(t *testing.T) {
	m := goodManifest()
	m.I18n = map[string]protocol.ManifestI18n{
		"zh_CN": {Name: "MyDNS 验证"},
		"ja_JP": {Description: "MyDNS で検証する"},
	}
	report, err := Lint(writeLintFixture(t, lintFixture{manifest: m}))
	require.NoError(t, err)
	assert.Empty(t, report.Findings, "%+v", report.Findings)

	m.I18n["zh"] = protocol.ManifestI18n{Name: "MyDNS"}
	m.I18n["pt-BR"] = protocol.ManifestI18n{Name: "MyDNS"}
	report, err = Lint(writeLintFixture(t, lintFixture{manifest: m}))
	require.NoError(t, err)
	assert.True(t, report.HasErrors())
	var locales []string
	for _, f := range report.Findings {
		if f.Rule == RuleManifestI18n {
			assert.Equal(t, LevelError, f.Level)
			locales = append(locales, f.Message)
		}
	}
	require.Len(t, locales, 2)
	assert.Contains(t, locales[0], `"pt-BR"`)
	assert.Contains(t, locales[1], `"zh"`)
}

func TestLintReservedNamespaceWarns(t *testing.T) {
	m := goodManifest()
	m.ID = "com.nginxui.example"
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.False(t, report.HasErrors())
	assertHasFinding(t, report, LevelWarning, RuleReservedNamespace)
}

func TestLintUnknownPermissionIsError(t *testing.T) {
	m := goodManifest()
	m.Permissions = append(m.Permissions, "not-a-real-permission")
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.True(t, report.HasErrors())
	assertHasFinding(t, report, LevelError, RuleManifestPermissions)
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
	assertHasFinding(t, report, LevelError, RuleSettingsOptions)
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
	assertHasFinding(t, report, LevelError, RulePackageLinks)
}

func TestLintDirectorySymlinkIsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevated privileges on windows")
	}
	dir := writeLintFixture(t, lintFixture{manifest: goodManifest()})
	require.NoError(t, os.Symlink(filepath.Join(dir, "README.md"), filepath.Join(dir, "evil-link")))

	report, err := Lint(dir)
	require.NoError(t, err)
	assertHasFinding(t, report, LevelError, RulePackageLinks)
}

// writeArchiveWithSymlink packages srcDir like BuildPackage, plus one
// dangling symlink entry, which a package may never contain whatever its target.
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

func TestLintChecksTheEmbeddedSignature(t *testing.T) {
	dir := writeLintFixture(t, lintFixture{manifest: goodManifest()})
	lintSigned := func(t *testing.T, key minisign.PrivateKey, mutate func(entry *tarEntry)) *LintReport {
		t.Helper()
		archive := filepath.Join(t.TempDir(), "plugin.tar.gz")
		require.NoError(t, BuildSignedPackage(dir, archive, key))
		if mutate != nil {
			archive = rewritePackage(t, archive, mutate)
		}
		report, err := Lint(archive)
		require.NoError(t, err)
		return report
	}
	release := useOfficialKey(t)
	_, community := newSigningKey(t)

	// A release signature is clean, a partner one is covered by
	// TestLintChecksThePartnerCertificate.
	assert.Empty(t, lintSigned(t, *release, nil).Findings)

	// A key the linter does not know is only a warning.
	report := lintSigned(t, community, nil)
	assertHasFinding(t, report, LevelWarning, RuleSignatureSigner)
	assert.False(t, report.HasErrors(), "%+v", report.Findings)

	// A file changed after signing no longer matches plugin.sums.
	report = lintSigned(t, community, func(entry *tarEntry) {
		if entry.header.Name == "LICENSE" {
			entry.body = "changed"
		}
	})
	assertHasFinding(t, report, LevelError, RuleSignatureMismatch)

	// A line that breaks the layout.
	report = lintSigned(t, *release, func(entry *tarEntry) {
		if entry.header.Name == SumsFileName {
			entry.body = strings.TrimSuffix(entry.body, "\n")
		}
	})
	assertHasFinding(t, report, LevelError, RuleSignatureSums)

	// A signature that does not parse, or that a known key fails.
	report = lintSigned(t, *release, func(entry *tarEntry) {
		if entry.header.Name == SumsSignatureFileName {
			entry.body = "untrusted comment: nothing\n"
		}
	})
	assertHasFinding(t, report, LevelError, RuleSignatureMismatch)
	report = lintSigned(t, *release, func(entry *tarEntry) {
		if entry.header.Name == SumsSignatureFileName {
			entry.body = string(minisign.Sign(*release, []byte("something else")))
		}
	})
	assertHasFinding(t, report, LevelError, RuleSignatureMismatch)

	// Only one of the two files leaves the package unsigned.
	for _, name := range []string{SumsFileName, SumsSignatureFileName} {
		report = lintSigned(t, *release, func(entry *tarEntry) {
			if entry.header.Name == name {
				entry.header.Name = ""
			}
		})
		assertHasFinding(t, report, LevelWarning, RuleSignatureFiles)
		assert.False(t, report.HasErrors(), "%+v", report.Findings)
	}
}

// warningRules lists the rules of a report, which must all be warnings.
func warningRules(t *testing.T, report *LintReport) []string {
	t.Helper()
	rules := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		assert.Equal(t, LevelWarning, finding.Level, "%+v", finding)
		rules = append(rules, finding.Rule)
	}
	return rules
}

func TestLintChecksTheSignerCertificate(t *testing.T) {
	useMarketplace(t)
	_, primary := newSigningKey(t)
	signingPublic, signing := newSigningKey(t)
	_, stranger := newSigningKey(t)
	id := goodManifest().ID
	certificate := signerFiles(t, signingPublic, id, primary)

	lintSigned := func(t *testing.T, files map[string]string, key minisign.PrivateKey) []string {
		t.Helper()
		dir := writeLintFixture(t, lintFixture{manifest: goodManifest()})
		for name, body := range files {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
		}
		archive := filepath.Join(t.TempDir(), "plugin.tar.gz")
		require.NoError(t, BuildSignedPackage(dir, archive, key))
		report, err := Lint(archive)
		require.NoError(t, err)
		return warningRules(t, report)
	}

	for name, testCase := range map[string]struct {
		files  map[string]string
		signer minisign.PrivateKey
		want   []string
	}{
		// The linter knows no primary key, so a community signature stays unknown.
		"valid certificate and signing key": {files: certificate, signer: signing, want: []string{RuleSignatureSigner}},
		"only plugin.signer": {
			files: map[string]string{SignerFileName: certificate[SignerFileName]}, signer: signing,
			want: []string{RuleSignerFiles, RuleSignatureSigner},
		},
		"certificate of another plugin": {
			files: signerFiles(t, signingPublic, "io.github.example.other", primary), signer: signing,
			want: []string{RuleSignerCertificate, RuleSignatureSigner},
		},
		"another key than the certificate names": {
			files: certificate, signer: stranger,
			want: []string{RuleSignerCertificate, RuleSignatureSigner},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.ElementsMatch(t, testCase.want, lintSigned(t, testCase.files, testCase.signer))
		})
	}
}

func TestLintChecksThePartnerCertificate(t *testing.T) {
	release := useOfficialKey(t)
	partnerPublic, partner := newSigningKey(t)
	_, stranger := newSigningKey(t)
	certificate := certify(t, partnerPublic, release, "example", "2099-12-31")

	// fixture writes a clean plugin directory plus files at its root.
	fixture := func(t *testing.T, files map[string]string) string {
		t.Helper()
		dir := writeLintFixture(t, lintFixture{manifest: goodManifest()})
		for name, body := range files {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
		}
		return dir
	}
	lintSigned := func(t *testing.T, dir string, key minisign.PrivateKey) *LintReport {
		t.Helper()
		archive := filepath.Join(t.TempDir(), "plugin.tar.gz")
		require.NoError(t, BuildSignedPackage(dir, archive, key))
		report, err := Lint(archive)
		require.NoError(t, err)
		return report
	}
	notAKey := "not a key\n"

	for name, testCase := range map[string]struct {
		files  map[string]string
		signer minisign.PrivateKey
		want   []string
	}{
		"valid certificate and partner signature": {files: certificate, signer: partner},
		"only plugin.partner": {
			files: map[string]string{PartnerFileName: certificate[PartnerFileName]}, signer: partner,
			want: []string{RulePartnerFiles, RuleSignatureSigner},
		},
		"only plugin.partner.minisig": {
			files: map[string]string{PartnerSignatureFileName: certificate[PartnerSignatureFileName]}, signer: partner,
			want: []string{RulePartnerFiles, RuleSignatureSigner},
		},
		"signed by a key that is not an official plugin key": {
			files: certify(t, partnerPublic, &stranger, "example", "2099-12-31"), signer: partner,
			want: []string{RulePartnerComment, RuleSignatureSigner},
		},
		"plugin.partner is no public key": {
			files: map[string]string{
				PartnerFileName:          notAKey,
				PartnerSignatureFileName: string(minisign.SignWithComments(*release, []byte(notAKey), "partner:example;expires:2099-12-31", "")),
			},
			signer: partner,
			want:   []string{RulePartnerComment, RuleSignatureSigner},
		},
		"trusted comment breaks the format": {
			files: map[string]string{
				PartnerFileName:          certificate[PartnerFileName],
				PartnerSignatureFileName: string(minisign.Sign(*release, []byte(certificate[PartnerFileName]))),
			},
			signer: partner,
			want:   []string{RulePartnerComment, RuleSignatureSigner},
		},
		"release signature with a certificate": {files: certificate, signer: *release, want: []string{RulePartnerCertificate}},
		"another key than the certificate names": {
			files: certificate, signer: stranger,
			want: []string{RulePartnerCertificate, RuleSignatureSigner},
		},
	} {
		t.Run(name, func(t *testing.T) {
			report := lintSigned(t, fixture(t, testCase.files), testCase.signer)
			assert.ElementsMatch(t, testCase.want, warningRules(t, report), "%+v", report.Findings)
		})
	}

	// A valid certificate in an unsigned directory signs nothing.
	report, err := Lint(fixture(t, certificate))
	require.NoError(t, err)
	assert.Equal(t, []string{RulePartnerCertificate}, warningRules(t, report))

	// Expiry follows the UTC date of the linter clock.
	certified := fixture(t, certificate)
	useNow(t, time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC))
	assert.Empty(t, lintSigned(t, certified, partner).Findings)
	useNow(t, time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC))
	report = lintSigned(t, certified, partner)
	assert.ElementsMatch(t, []string{RulePartnerCertificate, RuleSignatureSigner}, warningRules(t, report), "%+v", report.Findings)
	for _, finding := range report.Findings {
		if finding.Rule == RulePartnerCertificate {
			assert.Contains(t, finding.Message, "expired")
		}
	}

	// A certificate without an expiry never expires for the linter.
	lasting := fixture(t, certify(t, partnerPublic, release, "example", ""))
	assert.Empty(t, lintSigned(t, lasting, partner).Findings)
}

func webappLintManifest(chunks map[string]string) *protocol.Manifest {
	m := goodManifest()
	m.Webapp = &protocol.ManifestWebapp{BundlePath: "webapp/index.js", Chunks: chunks}
	return m
}

func writeWebappFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		target := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
		require.NoError(t, os.WriteFile(target, []byte(content), 0o644))
	}
}

func TestLintChunks(t *testing.T) {
	m := webappLintManifest(map[string]string{"search": "webapp/search.js", "dashboard": "webapp/chunks/dashboard.js"})
	dir := writeLintFixture(t, lintFixture{manifest: m})
	writeWebappFiles(t, dir, map[string]string{
		"webapp/index.js": "registerPlugin()", "webapp/search.js": "a", "webapp/chunks/dashboard.js": "b",
	})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.Empty(t, report.Findings, "%+v", report.Findings)
}

func TestLintChunkFailures(t *testing.T) {
	tests := []struct {
		name   string
		chunks map[string]string
		files  map[string]string
		level  Level
		rule   string
	}{
		{"missing file", map[string]string{"a": "webapp/a.js"}, nil, LevelError, RuleWebappChunks},
		{"empty file", map[string]string{"a": "webapp/a.js"}, map[string]string{"webapp/a.js": ""}, LevelError, RuleWebappChunkFiles},
		{"bad name", map[string]string{"A": "webapp/a.js"}, map[string]string{"webapp/a.js": "x"}, LevelError, RuleWebappChunks},
		{"not js", map[string]string{"a": "webapp/a.css"}, map[string]string{"webapp/a.css": "x"}, LevelError, RuleWebappChunks},
		{"the bundle", map[string]string{"a": "webapp/index.js"}, nil, LevelError, RuleWebappChunks},
		{"duplicate file", map[string]string{"a": "webapp/a.js", "b": "webapp/a.js"}, map[string]string{"webapp/a.js": "x"}, LevelError, RuleWebappChunks},
		{"outside the served directory", map[string]string{"a": "chunks/a.js"}, map[string]string{"chunks/a.js": "x"}, LevelWarning, RuleWebappChunkFiles},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeLintFixture(t, lintFixture{manifest: webappLintManifest(tt.chunks)})
			writeWebappFiles(t, dir, map[string]string{"webapp/index.js": "registerPlugin()"})
			writeWebappFiles(t, dir, tt.files)
			report, err := Lint(dir)
			require.NoError(t, err)
			assertHasFinding(t, report, tt.level, tt.rule)
		})
	}
}

func TestLintChunksNeedABundle(t *testing.T) {
	m := goodManifest()
	m.Webapp = &protocol.ManifestWebapp{Chunks: map[string]string{"a": "webapp/a.js"}}
	dir := writeLintFixture(t, lintFixture{manifest: m})
	writeWebappFiles(t, dir, map[string]string{"webapp/a.js": "x"})
	report, err := Lint(dir)
	require.NoError(t, err)
	assertHasFinding(t, report, LevelError, RuleWebappChunks)
}

func TestLintLogPathsChangedNeedsTheLogFilesPermission(t *testing.T) {
	m := goodManifest()
	m.Events = []string{protocol.EventLogPathsChanged}
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	assertHasFinding(t, report, LevelWarning, RuleEventPermission)
	assert.False(t, report.HasErrors())

	m.Permissions = append(m.Permissions, protocol.PermissionLogFiles)
	dir = writeLintFixture(t, lintFixture{manifest: m})
	report, err = Lint(dir)
	require.NoError(t, err)
	assert.Empty(t, report.Findings, "%+v", report.Findings)
}

func TestLintPermissionReasons(t *testing.T) {
	m := goodManifest()
	m.PermissionReasons = map[string]string{protocol.PermissionNetwork: "To call the API of MyDNS."}
	report, err := Lint(writeLintFixture(t, lintFixture{manifest: m}))
	require.NoError(t, err)
	for _, f := range report.Findings {
		assert.NotEqual(t, RuleManifestReasons, f.Rule)
	}

	m.PermissionReasons = map[string]string{protocol.PermissionKV: "To keep the zone list."}
	report, err = Lint(writeLintFixture(t, lintFixture{manifest: m}))
	require.NoError(t, err)
	assertHasFinding(t, report, LevelError, RuleManifestReasons)
}

func TestLintScreenshots(t *testing.T) {
	shot := func(id, path string) protocol.ManifestScreenshot {
		return protocol.ManifestScreenshot{ID: id, Path: path}
	}
	tests := []struct {
		name   string
		mutate func(m *protocol.Manifest)
		want   bool
	}{
		{"valid", func(m *protocol.Manifest) {
			m.Screenshots = []protocol.ManifestScreenshot{{ID: "dashboard", Path: "docs/1.png", DarkPath: "docs/1-dark.webp", Caption: "Dashboard"}}
			m.I18n = map[string]protocol.ManifestI18n{"zh_CN": {ScreenshotCaptions: map[string]string{"dashboard": "面板"}}}
		}, false},
		{"no id", func(m *protocol.Manifest) { m.Screenshots = []protocol.ManifestScreenshot{shot("", "docs/1.png")} }, true},
		{"bad id", func(m *protocol.Manifest) {
			m.Screenshots = []protocol.ManifestScreenshot{shot("Dash Board", "docs/1.png")}
		}, true},
		{"id used twice", func(m *protocol.Manifest) {
			m.Screenshots = []protocol.ManifestScreenshot{shot("a", "1.png"), shot("a", "2.png")}
		}, true},
		{"no path", func(m *protocol.Manifest) { m.Screenshots = []protocol.ManifestScreenshot{{ID: "a", Caption: "x"}} }, true},
		{"unsafe path", func(m *protocol.Manifest) { m.Screenshots = []protocol.ManifestScreenshot{shot("a", "../1.png")} }, true},
		{"not an image", func(m *protocol.Manifest) { m.Screenshots = []protocol.ManifestScreenshot{shot("a", "docs/1.gif")} }, true},
		{"dark path not an image", func(m *protocol.Manifest) {
			m.Screenshots = []protocol.ManifestScreenshot{{ID: "a", Path: "docs/1.png", DarkPath: "docs/1.svg"}}
		}, true},
		{"listed twice", func(m *protocol.Manifest) {
			m.Screenshots = []protocol.ManifestScreenshot{shot("a", "a.png"), shot("b", "a.png")}
		}, true},
		{"too many", func(m *protocol.Manifest) {
			for i := range maxScreenshots + 1 {
				m.Screenshots = append(m.Screenshots, shot("s"+strconv.Itoa(i), strconv.Itoa(i)+".png"))
			}
		}, true},
		{"caption of no screenshot", func(m *protocol.Manifest) {
			m.Screenshots = []protocol.ManifestScreenshot{shot("dashboard", "docs/1.png")}
			m.I18n = map[string]protocol.ManifestI18n{"zh_CN": {ScreenshotCaptions: map[string]string{"docs/1.png": "面板"}}}
		}, true},
		{"long caption", func(m *protocol.Manifest) {
			m.Screenshots = []protocol.ManifestScreenshot{{ID: "a", Path: "docs/1.png", Caption: strings.Repeat("x", maxScreenshotCaptionRunes+1)}}
		}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := goodManifest()
			tc.mutate(m)
			report, err := Lint(writeLintFixture(t, lintFixture{manifest: m}))
			require.NoError(t, err)
			found := false
			for _, f := range report.Findings {
				found = found || f.Rule == RuleManifestScreenshots
			}
			assert.Equal(t, tc.want, found)
		})
	}
}

func TestLintConflicts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(m *protocol.Manifest)
		want   bool
	}{
		{"valid", func(m *protocol.Manifest) { m.Conflicts = []string{"official.other"} }, false},
		{"malformed id", func(m *protocol.Manifest) { m.Conflicts = []string{"nope"} }, true},
		{"itself", func(m *protocol.Manifest) { m.Conflicts = []string{m.ID} }, true},
		{"repeated", func(m *protocol.Manifest) { m.Conflicts = []string{"official.other", "official.other"} }, true},
		{"also required", func(m *protocol.Manifest) {
			m.Conflicts = []string{"official.other"}
			m.Requires = []protocol.ManifestRequirement{{ID: "official.other"}}
		}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := goodManifest()
			tc.mutate(m)
			report, err := Lint(writeLintFixture(t, lintFixture{manifest: m}))
			require.NoError(t, err)
			if tc.want {
				assertHasFinding(t, report, LevelError, RuleManifestConflicts)
				return
			}
			for _, f := range report.Findings {
				assert.NotEqual(t, RuleManifestConflicts, f.Rule)
			}
		})
	}
}
