package main

import (
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"

	"aead.dev/minisign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEncryptedPrivateKeyAndSignArchive(t *testing.T) {
	publicKey, privateKey, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	encryptedKey, err := minisign.EncryptKey("release-password", privateKey)
	require.NoError(t, err)

	loadedPrivateKey, err := loadPrivateKey(encryptedKey, "release-password")
	require.NoError(t, err)
	assert.Equal(t, privateKey.ID(), loadedPrivateKey.ID())

	publicKeyText, err := publicKey.MarshalText()
	require.NoError(t, err)
	require.NoError(t, requireTrustedSigner(loadedPrivateKey, []string{string(publicKeyText)}))

	archivePath := filepath.Join(t.TempDir(), "nginx-ui-linux-64.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, []byte("release archive"), 0o600))
	require.NoError(t, signArchive(loadedPrivateKey, archivePath))

	archive, err := os.Open(archivePath)
	require.NoError(t, err)
	defer archive.Close()
	reader := minisign.NewReader(archive)
	_, err = io.Copy(io.Discard, reader)
	require.NoError(t, err)
	signature, err := os.ReadFile(archivePath + ".minisig")
	require.NoError(t, err)
	assert.True(t, reader.Verify(publicKey, signature))
}

func TestRequireTrustedSignerRejectsUnknownKey(t *testing.T) {
	trustedPublicKey, _, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	_, unknownPrivateKey, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	trustedPublicKeyText, err := trustedPublicKey.MarshalText()
	require.NoError(t, err)

	err = requireTrustedSigner(unknownPrivateKey, []string{string(trustedPublicKeyText)})
	assert.ErrorContains(t, err, "is not in the pinned release keys")
}
