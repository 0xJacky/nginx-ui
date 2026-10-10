package plugin

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"aead.dev/minisign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// signerFiles are the certificate files of signing for id, issued by primary.
func signerFiles(t *testing.T, signing minisign.PublicKey, id string, primary minisign.PrivateKey) map[string]string {
	t.Helper()
	signer, signature, err := NewSignerCertificate([]byte(encodeKey(t, signing)), id, primary)
	require.NoError(t, err)
	return map[string]string{SignerFileName: string(signer), SignerSignatureFileName: string(signature)}
}

// signedByCertificatePackage builds a webapp package with extra files, signed by signer.
func signedByCertificatePackage(t *testing.T, id string, certificate map[string]string, signer *minisign.PrivateKey) string {
	t.Helper()
	extra := map[string]string{"webapp/main.js": "export default {}", "README.md": "# readme\n"}
	for name, content := range certificate {
		extra[name] = content
	}
	return buildSignedTestPackage(t, marketplaceManifest(id, "1.0.0"), extra, signer)
}

// extractedSignerTrust unpacks a package and derives its trust with the
// catalog key and revoked signing keys of its entry.
func extractedSignerTrust(t *testing.T, archive, authorKey string, revoked []string) (packageTrust, error) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "payload")
	_, err := ExtractPackage(archive, dir)
	require.NoError(t, err)
	return verifyPackageSignature(dir, authorKey, revoked, nil)
}

func keyIDOf(key minisign.PublicKey) string {
	return fmt.Sprintf("%016X", key.ID())
}

func TestSignerCertificateMakesASigningKeyStandForThePrimary(t *testing.T) {
	useMarketplace(t)
	const id = "io.github.example.demo"
	primaryPublic, primary := newSigningKey(t)
	signingPublic, signing := newSigningKey(t)
	_, stranger := newSigningKey(t)
	authorKey := encodeKey(t, primaryPublic)

	for name, testCase := range map[string]struct {
		certificate map[string]string
		signer      *minisign.PrivateKey
		authorKey   string
		revoked     []string
		want        string
	}{
		"certified signing key": {certificate: signerFiles(t, signingPublic, id, primary), signer: &signing,
			authorKey: authorKey, want: TrustCommunity},
		"revoked signing key": {certificate: signerFiles(t, signingPublic, id, primary), signer: &signing,
			authorKey: authorKey, revoked: []string{strings.ToLower(keyIDOf(signingPublic))}, want: TrustUnsigned},
		"certificate of another plugin": {certificate: signerFiles(t, signingPublic, "io.github.example.other", primary),
			signer: &signing, authorKey: authorKey, want: TrustUnsigned},
		"certificate of another primary": {certificate: signerFiles(t, signingPublic, id, stranger), signer: &signing,
			authorKey: authorKey, want: TrustUnsigned},
		"no catalog key": {certificate: signerFiles(t, signingPublic, id, primary), signer: &signing, want: TrustUnsigned},
		"no certificate": {signer: &signing, authorKey: authorKey, want: TrustUnsigned},
		"incomplete certificate": {certificate: map[string]string{SignerFileName: encodeKey(t, signingPublic) + "\n"},
			signer: &signing, authorKey: authorKey, want: TrustUnsigned},
		"primary signs itself": {certificate: signerFiles(t, signingPublic, id, primary), signer: &primary,
			authorKey: authorKey, want: TrustCommunity},
	} {
		t.Run(name, func(t *testing.T) {
			archive := signedByCertificatePackage(t, id, testCase.certificate, testCase.signer)
			trust, err := extractedSignerTrust(t, archive, testCase.authorKey, testCase.revoked)
			require.NoError(t, err)
			assert.Equal(t, testCase.want, trust.Trust)
			if testCase.want == TrustCommunity {
				// The install records the primary key, whichever key signed.
				assert.Equal(t, strings.TrimSpace(authorKey), trust.AuthorKey)
				assert.Equal(t, fmt.Sprintf("%016X", testCase.signer.ID()), trust.Signer)
			}
		})
	}
}

func TestSignerCertificateFromATrustedPublisher(t *testing.T) {
	useMarketplace(t)
	const id = "io.github.example.demo"
	primaryPublic, primary := newSigningKey(t)
	trustKey(t, primaryPublic)
	signingPublic, signing := newSigningKey(t)

	archive := signedByCertificatePackage(t, id, signerFiles(t, signingPublic, id, primary), &signing)
	trust, err := extractedSignerTrust(t, archive, "", nil)
	require.NoError(t, err)
	assert.Equal(t, TrustCommunity, trust.Trust)
	assert.Equal(t, publisherKey(t, primaryPublic), trust.AuthorKey)
}

func TestReadSignerCertificateReportsWhatIsWrong(t *testing.T) {
	useMarketplace(t)
	const id = "io.github.example.demo"
	primaryPublic, primary := newSigningKey(t)
	signingPublic, signing := newSigningKey(t)
	primaries := []string{encodeKey(t, primaryPublic)}

	unpack := func(certificate map[string]string) string {
		dir := filepath.Join(t.TempDir(), "payload")
		_, err := ExtractPackage(signedByCertificatePackage(t, id, certificate, &signing), dir)
		require.NoError(t, err)
		return dir
	}

	certificate, err := readSignerCertificate(unpack(nil), primaries, nil)
	require.NoError(t, err)
	assert.Nil(t, certificate)

	certificate, err = readSignerCertificate(unpack(signerFiles(t, signingPublic, id, primary)), primaries, nil)
	require.NoError(t, err)
	assert.Equal(t, keyIDOf(signingPublic), certificate.KeyID)
	assert.Equal(t, id, certificate.PluginID)

	_, err = readSignerCertificate(unpack(map[string]string{SignerFileName: encodeKey(t, signingPublic) + "\n"}), primaries, nil)
	assert.ErrorIs(t, err, errSignerIncomplete)

	_, err = readSignerCertificate(unpack(signerFiles(t, signingPublic, "io.github.example.other", primary)), primaries, nil)
	assert.ErrorIs(t, err, errSignerPlugin)

	_, err = readSignerCertificate(unpack(signerFiles(t, signingPublic, id, primary)), primaries, []string{keyIDOf(signingPublic)})
	assert.ErrorIs(t, err, errSignerRevoked)
}

func TestNewSignerCertificateRefusesBadInput(t *testing.T) {
	primaryPublic, primary := newSigningKey(t)
	signingPublic, _ := newSigningKey(t)

	_, _, err := NewSignerCertificate([]byte(encodeKey(t, primaryPublic)), "io.github.example.demo", primary)
	assert.ErrorContains(t, err, "primary key itself")

	_, _, err = NewSignerCertificate([]byte(encodeKey(t, signingPublic)), "Not An Id", primary)
	assert.ErrorContains(t, err, "not valid")

	_, _, err = NewSignerCertificate([]byte("not a key"), "io.github.example.demo", primary)
	assert.ErrorContains(t, err, "signing key")
}

func TestInspectNamesTheCertifiedSigningKey(t *testing.T) {
	useMarketplace(t)
	const id = "io.github.example.demo"
	_, primary := newSigningKey(t)
	signingPublic, signing := newSigningKey(t)
	manager := newTestManager(t)

	result, err := manager.Inspect(signedByCertificatePackage(t, id, signerFiles(t, signingPublic, id, primary), &signing))
	require.NoError(t, err)
	assert.Equal(t, keyIDOf(signingPublic), result.CertifiedSigner)

	result, err = manager.Inspect(signedByCertificatePackage(t, id, nil, &signing))
	require.NoError(t, err)
	assert.Empty(t, result.CertifiedSigner)
}
