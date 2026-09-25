package cmd

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/internal/pkgsign"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeSigningKey stores an unencrypted minisign secret key and returns its
// path and the public key text.
func writeSigningKey(t *testing.T) (string, string) {
	t.Helper()
	public, private, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	encodedPrivate, err := private.MarshalText()
	require.NoError(t, err)
	encodedPublic, err := public.MarshalText()
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "plugin.key")
	require.NoError(t, os.WriteFile(path, encodedPrivate, 0o600))
	return path, string(encodedPublic)
}

// writePluginSource writes a minimal plugin directory.
func writePluginSource(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "plugin")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "webapp"), 0o755))
	manifest := &protocol.Manifest{
		ID:         "com.example.cli",
		Name:       "CLI",
		Version:    "1.0.0",
		APIVersion: protocol.APIVersion,
		Webapp:     &protocol.ManifestWebapp{BundlePath: "webapp/main.js"},
	}
	encoded, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, plugin.ManifestFileName), encoded, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "webapp", "main.js"), []byte("export default {}"), 0o644))
	return dir
}

// assertSignedBy extracts a package and verifies its embedded signature.
func assertSignedBy(t *testing.T, archive, publicKey string) {
	t.Helper()
	payload := filepath.Join(t.TempDir(), "payload")
	_, err := plugin.ExtractPackage(archive, payload)
	require.NoError(t, err)
	sums, err := os.ReadFile(filepath.Join(payload, plugin.SumsFileName))
	require.NoError(t, err)
	signature, err := os.ReadFile(filepath.Join(payload, plugin.SumsSignatureFileName))
	require.NoError(t, err)
	_, err = pkgsign.VerifyBytes(sums, signature, []string{publicKey})
	assert.NoError(t, err)
}

func TestPackAndSignPlugin(t *testing.T) {
	keyPath, publicKey := writeSigningKey(t)
	t.Setenv(signPasswordEnv, "")
	source := writePluginSource(t)
	ctx := context.Background()

	// pack without a key builds an unsigned package.
	unsigned := filepath.Join(t.TempDir(), "unsigned.tar.gz")
	require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "pack", source, unsigned}))
	payload := filepath.Join(t.TempDir(), "payload")
	_, err := plugin.ExtractPackage(unsigned, payload)
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(payload, plugin.SumsFileName))

	// pack with a key signs it, the flag may follow the arguments.
	signed := filepath.Join(t.TempDir(), "signed.tar.gz")
	require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "pack", source, signed, "--key", keyPath}))
	assertSignedBy(t, signed, publicKey)

	// sign signs an existing package in place.
	require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "sign", "--key", keyPath, unsigned}))
	assertSignedBy(t, unsigned, publicKey)

	// sign needs a key.
	assert.Error(t, PluginCommand.Run(ctx, []string{"plugin", "sign", unsigned}))
}

func TestLoadSigningKeyReadsAnUnencryptedKey(t *testing.T) {
	_, private, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "plugin.key")
	require.NoError(t, os.WriteFile(path, []byte("not a key"), 0o600))
	_, err = loadSigningKey(path)
	assert.Error(t, err)

	encoded, err := private.MarshalText()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, encoded, 0o600))
	key, err := loadSigningKey(path)
	require.NoError(t, err)
	assert.Equal(t, private.ID(), key.ID())
}
