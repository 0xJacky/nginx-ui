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
