package cmd

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
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

// captureStdout returns what run printed to standard output.
func captureStdout(t *testing.T, run func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	previous := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = previous }()

	run()
	require.NoError(t, writer.Close())
	printed, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(printed)
}

func TestCertifyPartnerWithoutExpiry(t *testing.T) {
	releaseKeyPath, releasePublic := writeSigningKey(t)
	_, partnerPublic := writeSigningKey(t)
	t.Setenv(signPasswordEnv, "")
	publicKeyPath := filepath.Join(t.TempDir(), "partner.pub")
	require.NoError(t, os.WriteFile(publicKeyPath, []byte(partnerPublic+"\n"), 0o644))
	out := t.TempDir()

	printed := captureStdout(t, func() {
		require.NoError(t, PluginCommand.Run(context.Background(), []string{"plugin", "certify", publicKeyPath,
			"--key", releaseKeyPath, "--name", "acme", "--out", out}))
	})
	assert.Contains(t, printed, "certified partner acme with no expiry")

	// The trusted comment carries the name alone.
	partner, err := os.ReadFile(filepath.Join(out, plugin.PartnerFileName))
	require.NoError(t, err)
	signature, err := os.ReadFile(filepath.Join(out, plugin.PartnerSignatureFileName))
	require.NoError(t, err)
	_, err = pkgsign.VerifyBytes(partner, signature, []string{releasePublic})
	require.NoError(t, err)
	var parsed minisign.Signature
	require.NoError(t, parsed.UnmarshalText(signature))
	assert.Equal(t, "partner:acme", parsed.TrustedComment)
}

func TestCertifyPartner(t *testing.T) {
	releaseKeyPath, releasePublic := writeSigningKey(t)
	partnerKeyPath, partnerPublic := writeSigningKey(t)
	t.Setenv(signPasswordEnv, "")
	publicKeyPath := filepath.Join(t.TempDir(), "partner.pub")
	require.NoError(t, os.WriteFile(publicKeyPath, []byte(partnerPublic+"\n"), 0o644))
	out := filepath.Join(t.TempDir(), "certificate")
	ctx := context.Background()

	require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "certify", publicKeyPath,
		"--key", releaseKeyPath, "--name", "acme", "--expires", "2099-12-31", "--out", out}))

	// The official plugin key signed the exact partner key bytes, and the trusted
	// comment names the partner and the expiry.
	partner, err := os.ReadFile(filepath.Join(out, plugin.PartnerFileName))
	require.NoError(t, err)
	signature, err := os.ReadFile(filepath.Join(out, plugin.PartnerSignatureFileName))
	require.NoError(t, err)
	assert.Equal(t, partnerPublic+"\n", string(partner))
	_, err = pkgsign.VerifyBytes(partner, signature, []string{releasePublic})
	require.NoError(t, err)
	var parsed minisign.Signature
	require.NoError(t, parsed.UnmarshalText(signature))
	assert.Equal(t, "partner:acme;expires:2099-12-31", parsed.TrustedComment)

	// The certificate names the key the partner signs its packages with.
	var certified minisign.PublicKey
	require.NoError(t, certified.UnmarshalText(partner))
	partnerKey, err := loadSigningKey(partnerKeyPath)
	require.NoError(t, err)
	assert.Equal(t, partnerKey.ID(), certified.ID())

	// The two files ship inside the package the partner signs.
	source := writePluginSource(t)
	for _, name := range []string{plugin.PartnerFileName, plugin.PartnerSignatureFileName} {
		body, err := os.ReadFile(filepath.Join(out, name))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(source, name), body, 0o644))
	}
	archive := filepath.Join(t.TempDir(), "certified.tar.gz")
	require.NoError(t, PluginCommand.Run(ctx, []string{"plugin", "pack", source, archive, "--key", partnerKeyPath}))
	assertSignedBy(t, archive, partnerPublic)
	payload := filepath.Join(t.TempDir(), "payload")
	_, err = plugin.ExtractPackage(archive, payload)
	require.NoError(t, err)
	sums, err := os.ReadFile(filepath.Join(payload, plugin.SumsFileName))
	require.NoError(t, err)
	assert.Contains(t, string(sums), "  "+plugin.PartnerFileName+"\n")
	assert.Contains(t, string(sums), "  "+plugin.PartnerSignatureFileName+"\n")

	for name, args := range map[string][]string{
		"no key file":  {"--key", releaseKeyPath, "--name", "acme", "--expires", "2099-12-31"},
		"bad name":     {publicKeyPath, "--key", releaseKeyPath, "--name", "acme corp", "--expires", "2099-12-31"},
		"bad date":     {publicKeyPath, "--key", releaseKeyPath, "--name", "acme", "--expires", "12/31/2099"},
		"past date":    {publicKeyPath, "--key", releaseKeyPath, "--name", "acme", "--expires", "2000-01-01"},
		"no signer":    {publicKeyPath, "--name", "acme", "--expires", "2099-12-31"},
		"not a key":    {releaseKeyPath, "--key", releaseKeyPath, "--name", "acme", "--expires", "2099-12-31"},
		"missing name": {publicKeyPath, "--key", releaseKeyPath, "--expires", "2099-12-31"},
	} {
		err = PluginCommand.Run(ctx, append([]string{"plugin", "certify", "--out", t.TempDir()}, args...))
		assert.Error(t, err, fmt.Sprintf("%s: %v", name, args))
	}
}
