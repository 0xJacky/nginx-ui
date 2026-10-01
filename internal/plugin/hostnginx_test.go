package plugin

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeNginx stands in for nginx -t and the reload. included lists the
// snippet files the configuration includes, which fail the test when gone.
type fakeNginx struct {
	reject   bool
	included []string
	reloads  int
}

func (n *fakeNginx) test() error {
	if n.reject {
		return errors.New("unknown directive \"nope\"")
	}
	for _, file := range n.included {
		if _, err := os.Stat(file); err != nil {
			return errors.New("open() " + file + " failed")
		}
	}
	return nil
}

func newNginxBackend(t *testing.T) (*hostBackend, *fakeNginx, string) {
	t.Helper()
	dir := t.TempDir()
	n := &fakeNginx{}
	b := &hostBackend{
		writer: config.GeneratedWriter{
			Test:   n.test,
			Reload: func() error { n.reloads++; return nil },
		},
		confDir: func() string { return dir },
	}
	return b, n, dir
}

func requireInvalidParams(t *testing.T, err error) {
	t.Helper()
	perr, ok := jsonrpc.AsProtocolError(err)
	require.True(t, ok, "want a protocol error, got %v", err)
	assert.Equal(t, protocol.CodeInvalidParams, perr.Code)
}

func TestNginxSnippetPutTestsAndReloads(t *testing.T) {
	b, n, dir := newNginxBackend(t)
	file := filepath.Join(dir, "snippets", "plugins", "io.github.example.cache", "static.conf")

	changed, err := b.NginxSnippetPut("io.github.example.cache", "static", "expires 1d;\n")
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, 1, n.reloads)
	content, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, "expires 1d;\n", string(content))

	// The same content changes nothing and reloads nothing.
	changed, err = b.NginxSnippetPut("io.github.example.cache", "static", "expires 1d;\n")
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, 1, n.reloads)

	// A configuration nginx rejects puts the previous snippet back.
	n.reject = true
	_, err = b.NginxSnippetPut("io.github.example.cache", "static", "nope;\n")
	requireInvalidParams(t, err)
	assert.Contains(t, err.Error(), "unknown directive")
	content, err = os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, "expires 1d;\n", string(content))
	assert.Equal(t, 1, n.reloads)

	snippets, err := b.NginxSnippetList("io.github.example.cache")
	require.NoError(t, err)
	assert.Equal(t, []protocol.HostNginxSnippet{{
		Name:    "static",
		Include: "include snippets/plugins/io.github.example.cache/static.conf;",
	}}, snippets)
}

func TestNginxSnippetPutValidatesItsInput(t *testing.T) {
	b, _, _ := newNginxBackend(t)
	for _, name := range []string{"", "-a", "A", "a.b", "a/b", "../a", strings.Repeat("a", 65)} {
		_, err := b.NginxSnippetPut("io.github.example.cache", name, "")
		requireInvalidParams(t, err)
	}
	_, err := b.NginxSnippetPut("io.github.example.cache", "big", strings.Repeat("#", maxSnippetBytes+1))
	requireInvalidParams(t, err)
	_, err = b.NginxSnippetPut("io.github.example.cache", "bytes", "\xff")
	requireInvalidParams(t, err)

	for i := range maxSnippetsPerPlugin {
		_, err = b.NginxSnippetPut("io.github.example.cache", "s"+strings.Repeat("x", i), "")
		require.NoError(t, err)
	}
	_, err = b.NginxSnippetPut("io.github.example.cache", "one-too-many", "")
	requireInvalidParams(t, err)
	// Replacing an existing snippet is still allowed at the limit.
	_, err = b.NginxSnippetPut("io.github.example.cache", "s", "# again\n")
	require.NoError(t, err)
}

func TestNginxSnippetDeleteKeepsAnIncludedSnippet(t *testing.T) {
	b, n, dir := newNginxBackend(t)
	_, err := b.NginxSnippetPut("io.github.example.cache", "static", "expires 1d;\n")
	require.NoError(t, err)
	file := filepath.Join(dir, "snippets", "plugins", "io.github.example.cache", "static.conf")

	n.included = []string{file}
	_, err = b.NginxSnippetDelete("io.github.example.cache", "static")
	requireInvalidParams(t, err)
	assert.FileExists(t, file)

	n.included = nil
	removed, err := b.NginxSnippetDelete("io.github.example.cache", "static")
	require.NoError(t, err)
	assert.True(t, removed)
	assert.NoFileExists(t, file)

	removed, err = b.NginxSnippetDelete("io.github.example.cache", "static")
	require.NoError(t, err)
	assert.False(t, removed)
}

func TestRemoveSnippetsEmptiesWhatIsStillIncluded(t *testing.T) {
	b, n, dir := newNginxBackend(t)
	pluginDir := filepath.Join(dir, "snippets", "plugins", "io.github.example.cache")
	for _, name := range []string{"kept", "gone"} {
		_, err := b.NginxSnippetPut("io.github.example.cache", name, "expires 1d;\n")
		require.NoError(t, err)
	}
	kept := filepath.Join(pluginDir, "kept.conf")
	n.included = []string{kept}

	require.NoError(t, b.removeSnippets("io.github.example.cache"))
	assert.NoFileExists(t, filepath.Join(pluginDir, "gone.conf"))
	content, err := os.ReadFile(kept)
	require.NoError(t, err)
	assert.Equal(t, "# The plugin io.github.example.cache was uninstalled.\n", string(content))

	n.included = nil
	require.NoError(t, b.removeSnippets("io.github.example.cache"))
	assert.NoDirExists(t, pluginDir)

	// A plugin without snippets has nothing to remove.
	require.NoError(t, b.removeSnippets("io.github.example.other"))
}

func writeConfTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

func TestNginxConfigListAndGet(t *testing.T) {
	b, _, dir := newNginxBackend(t)
	outside := t.TempDir()
	writeConfTree(t, dir, map[string]string{
		"nginx.conf":                 "events {}",
		"mime.types":                 "types {}",
		"conf.d/gzip.conf":           "gzip on;",
		"sites-available/example":    "server {}",
		"streams-available/dns":      "server {}",
		"ssl/example.key":            "PRIVATE",
		"ssl/example.conf.key":       "PRIVATE",
		".htpasswd":                  "user:hash",
		"sites-available/deep/x.txt": "no",
	})
	writeConfTree(t, outside, map[string]string{"secret.conf": "outside"})
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret.conf"), filepath.Join(dir, "conf.d", "linked.conf")))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "linked-dir")))

	files, err := b.NginxConfigList()
	require.NoError(t, err)
	assert.Equal(t, []string{
		"conf.d/gzip.conf",
		"nginx.conf",
		"sites-available/example",
		"streams-available/dns",
	}, files)

	content, err := b.NginxConfigGet("sites-available/example")
	require.NoError(t, err)
	assert.Equal(t, "server {}", content)

	for _, rel := range []string{
		"", "/etc/passwd", "../outside.conf", "conf.d/../nginx.conf", "./nginx.conf", "conf.d\\gzip.conf",
		"ssl/example.key", ".htpasswd", "mime.types", "missing.conf",
		"conf.d/linked.conf", "linked-dir/secret.conf",
	} {
		_, err = b.NginxConfigGet(rel)
		requireInvalidParams(t, err)
	}
}

// useNginxConfDir points the nginx package at dir for one test.
func useNginxConfDir(t *testing.T, dir string) {
	t.Helper()
	original := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = dir
	t.Cleanup(func() { settings.NginxSettings.ConfigDir = original })
}

func TestSitesList(t *testing.T) {
	db := setupPluginTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Site{}, &model.Namespace{}))
	dir := t.TempDir()
	useNginxConfDir(t, dir)
	writeConfTree(t, dir, map[string]string{
		"sites-available/b.test": "server {}",
		"sites-available/a.test": "server {}",
	})
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sites-enabled"), 0o755))
	require.NoError(t, os.Symlink(filepath.Join(dir, "sites-available", "a.test"), filepath.Join(dir, "sites-enabled", "a.test")))

	sites, err := (&hostBackend{}).SitesList()
	require.NoError(t, err)
	assert.Equal(t, []protocol.HostSite{
		{Name: "a.test", Status: "enabled", URLs: []string{}, ConfigFile: "sites-available/a.test"},
		{Name: "b.test", Status: "disabled", URLs: []string{}, ConfigFile: "sites-available/b.test"},
	}, sites)
}

func TestCertsListLeavesTheKeyOut(t *testing.T) {
	db := setupPluginTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Cert{}))
	dir := t.TempDir()
	useNginxConfDir(t, dir)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	notBefore := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "a.test"},
		Issuer:       pkix.Name{CommonName: "a.test"},
		DNSNames:     []string{"a.test"},
		NotBefore:    notBefore,
		NotAfter:     notBefore.AddDate(0, 3, 0),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	certPath := filepath.Join(dir, "ssl", "a.test", "fullchain.cer")
	writeConfTree(t, dir, map[string]string{
		"ssl/a.test/fullchain.cer": string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
	})

	require.NoError(t, db.Create(&model.Cert{
		Name: "b", Domains: []string{"b.test"}, AutoCert: model.AutoCertDisabled,
		SSLCertificatePath: filepath.Join(dir, "missing.cer"),
	}).Error)
	require.NoError(t, db.Create(&model.Cert{
		Name: "a", Domains: []string{"a.test"}, AutoCert: model.AutoCertEnabled, ChallengeMethod: "dns01",
		KeyType: "P256", SSLCertificatePath: certPath, SSLCertificateKeyPath: filepath.Join(dir, "ssl", "a.test", "private.key"),
	}).Error)

	certs, err := (&hostBackend{}).CertsList()
	require.NoError(t, err)
	require.Len(t, certs, 2)
	assert.Equal(t, protocol.HostCert{
		ID: certs[0].ID, Name: "a", Domains: []string{"a.test"}, AutoRenew: true, ChallengeMethod: "dns01", KeyType: "P256",
		NotBefore: "2026-09-01T00:00:00Z", NotAfter: "2026-12-01T00:00:00Z", Issuer: "a.test",
	}, certs[0])
	// A file that cannot be read leaves the dates empty.
	assert.Equal(t, "b", certs[1].Name)
	assert.Empty(t, certs[1].NotAfter)

	encoded, err := json.Marshal(certs)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "private")
}
