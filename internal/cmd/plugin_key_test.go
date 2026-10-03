package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readPublicKey reads a public key file written by plugin key.
func readPublicKey(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(body)
}

// packagedCertificate returns the plugin id and signing key id of the
// certificate a package carries.
func packagedCertificate(t *testing.T, archive string) (string, string) {
	t.Helper()
	payload := filepath.Join(t.TempDir(), "payload")
	_, err := plugin.ExtractPackage(archive, payload)
	require.NoError(t, err)
	signer, err := os.ReadFile(filepath.Join(payload, plugin.SignerFileName))
	require.NoError(t, err)
	signature, err := os.ReadFile(filepath.Join(payload, plugin.SignerSignatureFileName))
	require.NoError(t, err)
	id, key, err := plugin.InspectSignerCertificate(signer, signature)
	require.NoError(t, err)
	return id, key.KeyID
}

func TestPluginKeyInitAndPack(t *testing.T) {
	t.Setenv(signPasswordEnv, "")
	t.Setenv(primaryPasswordEnv, "")
	ctx := context.Background()
	keys := t.TempDir()
	source := writePluginSource(t)

	require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "key", "init", "--id", "com.example.cli", "--out", keys}))
	for _, name := range []string{primaryPublicFile, primarySecretFile, signingPublicFile, signingSecretFile, plugin.SignerFileName, plugin.SignerSignatureFileName} {
		assert.FileExists(t, filepath.Join(keys, name))
	}
	info, err := os.Stat(filepath.Join(keys, primarySecretFile))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	// A second init never overwrites the keys.
	assert.ErrorContains(t, PluginCommand.Run(ctx, []string{"plugin", "key", "init", "--id", "com.example.cli", "--out", keys}), "already exists")

	// pack finds the certificate next to the signing key and packs it.
	signed := filepath.Join(t.TempDir(), "signed.tar.gz")
	require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "pack", source, signed, "--key", filepath.Join(keys, signingSecretFile)}))
	assertSignedBy(t, signed, readPublicKey(t, filepath.Join(keys, signingPublicFile)))
	id, keyID := packagedCertificate(t, signed)
	assert.Equal(t, "com.example.cli", id)
	signing, err := loadSigningKey(filepath.Join(keys, signingSecretFile))
	require.NoError(t, err)
	assert.Equal(t, keyID, fmtKeyID(signing.ID()))

	// sign adds it to an unsigned package too.
	unsigned := filepath.Join(t.TempDir(), "unsigned.tar.gz")
	require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "pack", source, unsigned}))
	require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "sign", "--key", filepath.Join(keys, signingSecretFile), unsigned}))
	_, keyID = packagedCertificate(t, unsigned)
	assert.Equal(t, fmtKeyID(signing.ID()), keyID)

	// Signing with the primary key next to the certificate is refused.
	err = PluginCommand.Run(ctx, []string{"plugin", "pack", source, filepath.Join(t.TempDir(), "p.tar.gz"), "--key", filepath.Join(keys, primarySecretFile)})
	assert.ErrorContains(t, err, "not the primary key")
}

func TestPluginKeyRefusesACertificateOfAnotherPlugin(t *testing.T) {
	t.Setenv(signPasswordEnv, "")
	t.Setenv(primaryPasswordEnv, "")
	ctx := context.Background()
	keys := t.TempDir()
	require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "key", "init", "--id", "com.example.other", "--out", keys}))
	err := PluginCommand.Run(ctx, []string{"plugin", "pack", writePluginSource(t), filepath.Join(t.TempDir(), "p.tar.gz"),
		"--key", filepath.Join(keys, signingSecretFile)})
	assert.ErrorContains(t, err, "is for com.example.other")
}

func TestPluginKeyRotateAndCertify(t *testing.T) {
	t.Setenv(signPasswordEnv, "")
	t.Setenv(primaryPasswordEnv, "")
	ctx := context.Background()
	keys := t.TempDir()
	require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "key", "init", "--id", "com.example.cli", "--out", keys}))
	primary := filepath.Join(keys, primarySecretFile)
	first := readPublicKey(t, filepath.Join(keys, signingPublicFile))

	assert.ErrorContains(t, PluginCommand.Run(ctx, []string{"plugin", "key", "rotate", "--id", "com.example.cli", "--primary", primary, "--out", keys}), "already exists")
	require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "key", "rotate", "--id", "com.example.cli", "--primary", primary, "--out", keys, "--force"}))
	assert.NotEqual(t, first, readPublicKey(t, filepath.Join(keys, signingPublicFile)))

	// certify issues a certificate for an existing key, for another plugin.
	other := t.TempDir()
	require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "key", "certify", "--id", "com.example.second", "--primary", primary,
		"--out", other, filepath.Join(keys, signingPublicFile)}))
	signer, err := os.ReadFile(filepath.Join(other, plugin.SignerFileName))
	require.NoError(t, err)
	signature, err := os.ReadFile(filepath.Join(other, plugin.SignerSignatureFileName))
	require.NoError(t, err)
	id, _, err := plugin.InspectSignerCertificate(signer, signature)
	require.NoError(t, err)
	assert.Equal(t, "com.example.second", id)
}

func TestPluginKeyRevokePrintsTheKeyID(t *testing.T) {
	t.Setenv(signPasswordEnv, "")
	t.Setenv(primaryPasswordEnv, "")
	ctx := context.Background()
	keys := t.TempDir()
	require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "key", "init", "--id", "com.example.cli", "--out", keys}))
	signing, err := loadSigningKey(filepath.Join(keys, signingSecretFile))
	require.NoError(t, err)

	printed := captureStdout(t, func() {
		require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "key", "revoke", filepath.Join(keys, signingPublicFile)}))
	})
	assert.Contains(t, printed, `"revoked_signers": ["`+fmtKeyID(signing.ID())+`"]`)

	printed = captureStdout(t, func() {
		require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "key", "revoke", "abcdef0123456789"}))
	})
	assert.Contains(t, printed, `"ABCDEF0123456789"`)
}

func fmtKeyID(id uint64) string {
	return fmt.Sprintf("%016X", id)
}
