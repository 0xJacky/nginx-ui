package upgrader

import (
	"bytes"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"

	"aead.dev/minisign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyArchiveSignatureSupportsKeyRotation(t *testing.T) {
	oldPublicKey, _, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	newPublicKey, newPrivateKey, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)

	oldKeyText, err := oldPublicKey.MarshalText()
	require.NoError(t, err)
	newKeyText, err := newPublicKey.MarshalText()
	require.NoError(t, err)
	setTrustedMinisignKeysForTest(t, string(oldKeyText), string(newKeyText))

	archivePath := filepath.Join(t.TempDir(), "nginx-ui.tar.gz")
	archiveContent := []byte("signed release archive")
	require.NoError(t, os.WriteFile(archivePath, archiveContent, 0o600))
	signature := signArchiveContent(t, newPrivateKey, archiveContent)

	keyID, err := verifyArchiveSignature(archivePath, signature)
	require.NoError(t, err)
	assert.Equal(t, newPublicKey.ID(), keyID)
}

func TestVerifyArchiveSignatureRejectsTamperingAndUnknownKeys(t *testing.T) {
	trustedPublicKey, trustedPrivateKey, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	unknownPublicKey, unknownPrivateKey, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	trustedKeyText, err := trustedPublicKey.MarshalText()
	require.NoError(t, err)
	setTrustedMinisignKeysForTest(t, string(trustedKeyText))

	archivePath := filepath.Join(t.TempDir(), "nginx-ui.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, []byte("original"), 0o600))
	trustedSignature := signArchiveContent(t, trustedPrivateKey, []byte("original"))
	require.NoError(t, os.WriteFile(archivePath, []byte("tampered"), 0o600))
	_, err = verifyArchiveSignature(archivePath, trustedSignature)
	assert.ErrorIs(t, err, ErrSignatureInvalid)

	unknownSignature := signArchiveContent(t, unknownPrivateKey, []byte("tampered"))
	keyID, err := verifyArchiveSignature(archivePath, unknownSignature)
	assert.ErrorIs(t, err, ErrSignatureKeyUnknown)
	assert.Equal(t, unknownPublicKey.ID(), keyID)
}

func TestVerifyArchiveSignatureFailsWithoutTrustedKeys(t *testing.T) {
	setTrustedMinisignKeysForTest(t)
	_, privateKey, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	archivePath := filepath.Join(t.TempDir(), "nginx-ui.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, []byte("archive"), 0o600))
	_, err = verifyArchiveSignature(archivePath, signArchiveContent(t, privateKey, []byte("archive")))
	assert.ErrorIs(t, err, ErrTrustedSignatureKeysEmpty)
}

func TestPerformCoreUpgradeFailsClosedBeforeExtraction(t *testing.T) {
	publicKey, privateKey, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	publicKeyText, err := publicKey.MarshalText()
	require.NoError(t, err)
	setTrustedMinisignKeysForTest(t, string(publicKeyText))

	archivePath := filepath.Join(t.TempDir(), "nginx-ui.tar.gz")
	original := []byte("original archive")
	require.NoError(t, os.WriteFile(archivePath, original, 0o600))
	upgrader := &Upgrader{}
	upgrader.ExPath = filepath.Join(t.TempDir(), "nginx-ui")

	err = upgrader.PerformCoreUpgrade(archivePath)
	assert.ErrorIs(t, err, ErrSignatureEmpty)

	require.NoError(t, os.WriteFile(archivePath+".minisig", signArchiveContent(t, privateKey, original), 0o600))
	require.NoError(t, os.WriteFile(archivePath, []byte("tampered archive"), 0o600))
	err = upgrader.PerformCoreUpgrade(archivePath)
	assert.ErrorIs(t, err, ErrSignatureInvalid)
}

func signArchiveContent(t *testing.T, privateKey minisign.PrivateKey, content []byte) []byte {
	t.Helper()
	reader := minisign.NewReader(bytes.NewReader(content))
	_, err := io.Copy(io.Discard, reader)
	require.NoError(t, err)
	return reader.Sign(privateKey)
}

func setTrustedMinisignKeysForTest(t *testing.T, keys ...string) {
	t.Helper()
	original := trustedMinisignPublicKeys
	trustedMinisignPublicKeys = append([]string(nil), keys...)
	t.Cleanup(func() {
		trustedMinisignPublicKeys = original
	})
}
