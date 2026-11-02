package cert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// longIdentifierList returns count subdomains of domain whose joined directory
// name is well beyond the file name limit, like the list in issue #2001.
func longIdentifierList(domain string, count int) []string {
	identifiers := make([]string, 0, count)
	for i := range count {
		identifiers = append(identifiers, fmt.Sprintf("service-%02d-subdomain.%s", i, domain))
	}
	return identifiers
}

func TestCertificateDirNameKeepsShortListsUnchanged(t *testing.T) {
	got := certificateDirName([]string{"example.com", "www.example.com"}, certcrypto.EC256)
	assert.Equal(t, "example.com_www.example.com_EC256", got)
}

func TestCertificateDirNameBoundsLongLists(t *testing.T) {
	first := longIdentifierList("example.com", 12)
	second := longIdentifierList("example.net", 12)
	require.Greater(t, len(strings.Join(first, "_")), maxCertificateDirNameLength,
		"test premise: the joined list must exceed the file name limit")

	firstName := certificateDirName(first, certcrypto.EC256)
	secondName := certificateDirName(second, certcrypto.EC256)

	assert.LessOrEqual(t, len(firstName), maxCertificateDirNameLength)
	assert.True(t, strings.HasPrefix(firstName, first[0]+"_"), firstName)
	assert.True(t, strings.HasSuffix(firstName, "_EC256"), firstName)
	assert.Equal(t, firstName, certificateDirName(first, certcrypto.EC256), "the name must be stable")
	assert.NotEqual(t, firstName, secondName)
	assert.NotEqual(t, firstName, certificateDirName(first[:11], certcrypto.EC256),
		"lists sharing the first identifier must not collide")
}

func TestCertificateDirNameBoundsSingleLongIdentifier(t *testing.T) {
	label := strings.Repeat("a", 63)
	identifier := strings.Join([]string{label, label, label, label[:57], "com"}, ".")
	require.Len(t, identifier, 253, "test premise: the longest valid domain name")

	got := certificateDirName([]string{identifier}, certcrypto.RSA2048)
	assert.LessOrEqual(t, len(got), maxCertificateDirNameLength)
	assert.True(t, strings.HasSuffix(got, "_RSA2048"), got)
}

// TestCertificatePathsOfLongListsStayApart is the regression test for issue
// #2001: two certificates with long identifier lists both resolved to
// fullchain.cer and private.key in the nginx configuration directory and
// overwrote each other.
func TestCertificatePathsOfLongListsStayApart(t *testing.T) {
	confDir := useTempNginxConfDir(t)
	sslDir := filepath.Join(confDir, "ssl")

	first := &ConfigPayload{ServerName: longIdentifierList("example.com", 12), KeyType: certcrypto.EC256}
	second := &ConfigPayload{ServerName: longIdentifierList("example.net", 12), KeyType: certcrypto.EC256}

	for _, payload := range []*ConfigPayload{first, second} {
		assert.Equal(t, sslDir, filepath.Dir(payload.getCertificateDirPath()))
		assert.False(t, IsConfRootCertificatePath(payload.GetCertificatePath()), payload.GetCertificatePath())
		require.NoError(t, payload.mkCertificateDir())
	}
	assert.NotEqual(t, first.GetCertificatePath(), second.GetCertificatePath())
	assert.NotEqual(t, first.GetCertificateKeyPath(), second.GetCertificateKeyPath())
}

func TestMkCertificateDirRefusesTheConfigurationDirectory(t *testing.T) {
	confDir := useTempNginxConfDir(t)

	payload := &ConfigPayload{CertificateDir: confDir}
	assert.ErrorIs(t, payload.mkCertificateDir(), errCertificateDirIsConfRoot)
}

// TestUseExistingCertificatePathsIgnoresSharedConfRootFiles covers records
// written before the fix: they point at the files shared in the configuration
// directory, and pinning them would keep the certificates overwriting each
// other, so the renewal moves to the certificate's own directory instead.
func TestUseExistingCertificatePathsIgnoresSharedConfRootFiles(t *testing.T) {
	confDir := useTempNginxConfDir(t)
	certPath := filepath.Join(confDir, "fullchain.cer")
	keyPath := filepath.Join(confDir, "private.key")
	require.NoError(t, os.WriteFile(certPath, []byte("shared"), 0o644))
	require.NoError(t, os.WriteFile(keyPath, []byte("shared"), 0o600))

	payload := &ConfigPayload{ServerName: longIdentifierList("example.com", 12), KeyType: certcrypto.EC256}
	payload.UseExistingCertificatePaths(certPath, keyPath)

	assert.Empty(t, payload.SSLCertificatePath)
	assert.Empty(t, payload.SSLCertificateKeyPath)
	assert.Equal(t, filepath.Join(confDir, "ssl"), filepath.Dir(payload.getCertificateDirPath()))
}

// stubCertificateTargetGOOS makes the certificate directory follow the file
// name rules of goos regardless of the OS running the test.
func stubCertificateTargetGOOS(t *testing.T, goos string) {
	t.Helper()

	original := certificateTargetGOOS
	certificateTargetGOOS = func() string { return goos }
	t.Cleanup(func() { certificateTargetGOOS = original })
}

// TestWildcardCertificateDirFollowsTargetOS is the regression test for issue
// #2067: on Windows the wildcard directory name held "*", the path check
// rejected it, the path collapsed to the nginx configuration directory, and
// the issued certificate could not be saved. Linux keeps the literal "*" that
// existing installations store their certificates under.
func TestWildcardCertificateDirFollowsTargetOS(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{goos: "windows", want: "#.example.com_example.com_EC256"},
		{goos: "linux", want: "*.example.com_example.com_EC256"},
		{goos: "darwin", want: "*.example.com_example.com_EC256"},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			confDir := useTempNginxConfDir(t)
			stubCertificateTargetGOOS(t, tt.goos)

			payload := &ConfigPayload{
				ServerName: []string{"*.example.com", "example.com"},
				KeyType:    certcrypto.EC256,
			}
			dir := payload.getCertificateDirPath()

			assert.Equal(t, filepath.Join(confDir, "ssl", tt.want), dir)
			assert.False(t, isConfRootCertificateDir(dir), dir)
			require.NoError(t, payload.mkCertificateDir())
			info, err := os.Stat(payload.CertificateDir)
			require.NoError(t, err)
			assert.True(t, info.IsDir())
			assert.Equal(t, filepath.Join(dir, "fullchain.cer"), payload.GetCertificatePath())
			assert.Equal(t, filepath.Join(dir, "private.key"), payload.GetCertificateKeyPath())
		})
	}
}

func TestCertificateDirWithoutWildcardIsTheSameOnEveryTarget(t *testing.T) {
	for _, goos := range []string{"windows", "linux"} {
		t.Run(goos, func(t *testing.T) {
			confDir := useTempNginxConfDir(t)
			stubCertificateTargetGOOS(t, goos)

			payload := &ConfigPayload{
				ServerName: []string{"example.com", "www.example.com"},
				KeyType:    certcrypto.RSA2048,
			}

			assert.Equal(t, filepath.Join(confDir, "ssl", "example.com_www.example.com_RSA2048"),
				payload.getCertificateDirPath())
			require.NoError(t, payload.mkCertificateDir())
		})
	}
}

// TestWildcardCertificateDirKeepsStoredPath makes sure a renewal of a
// certificate whose record already points at a directory keeps writing there,
// whatever name the target would derive today.
func TestWildcardCertificateDirKeepsStoredPath(t *testing.T) {
	confDir := useTempNginxConfDir(t)
	stubCertificateTargetGOOS(t, "windows")

	storedDir := filepath.Join(confDir, "ssl", "legacy_wildcard_EC256")
	payload := &ConfigPayload{
		ServerName:         []string{"*.example.com"},
		KeyType:            certcrypto.EC256,
		SSLCertificatePath: filepath.Join(storedDir, "fullchain.cer"),
	}

	assert.Equal(t, storedDir, payload.getCertificateDirPath())
}

func TestTargetCertificateDirName(t *testing.T) {
	assert.Equal(t, "#.example.com_EC256", targetCertificateDirName("*.example.com_EC256", "windows"))
	assert.Equal(t, "*.example.com_EC256", targetCertificateDirName("*.example.com_EC256", "linux"))
	assert.Equal(t, "example.com_EC256", targetCertificateDirName("example.com_EC256", "windows"))
}
