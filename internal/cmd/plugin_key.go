package cmd

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/urfave/cli/v3"
)

// The files plugin key writes. The certificate files keep the names they have
// in a package.
const (
	primaryPublicFile  = "primary.pub"
	primarySecretFile  = "primary.key"
	signingPublicFile  = "signing.pub"
	signingSecretFile  = "signing.key"
	primaryPasswordEnv = "NGINX_UI_PLUGIN_PRIMARY_PASSWORD"
)

// keyIDPattern is a minisign key id in hex.
var keyIDPattern = regexp.MustCompile(`^[0-9A-Fa-f]{16}$`)

var pluginKeyCommand = &cli.Command{
	Name:  "key",
	Usage: "Manage the primary key and the signing keys of a community plugin",
	Commands: []*cli.Command{
		{
			Name:   "init",
			Usage:  "Create a primary key, a signing key and the certificate that links them",
			Action: InitPluginKeys,
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "id", Usage: "plugin id the signing key may sign", Required: true},
				&cli.StringFlag{Name: "out", Value: ".", Usage: "directory to write the keys and the certificate into"},
			},
		},
		{
			Name:   "rotate",
			Usage:  "Create a new signing key and certify it with the primary key",
			Action: RotatePluginKey,
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "id", Usage: "plugin id the signing key may sign", Required: true},
				&cli.StringFlag{Name: "primary", Usage: "primary secret key file", Required: true},
				&cli.StringFlag{Name: "out", Value: ".", Usage: "directory to write the signing key and the certificate into"},
				&cli.BoolFlag{Name: "force", Usage: "replace the signing key and certificate files already in --out"},
			},
		},
		{
			Name:      "certify",
			Usage:     "Certify an existing signing key for a plugin with the primary key",
			ArgsUsage: "<signing-public-key-file>",
			Action:    CertifyPluginKey,
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "id", Usage: "plugin id the signing key may sign", Required: true},
				&cli.StringFlag{Name: "primary", Usage: "primary secret key file", Required: true},
				&cli.StringFlag{Name: "out", Value: ".", Usage: "directory to write plugin.signer and plugin.signer.minisig into"},
				&cli.BoolFlag{Name: "force", Usage: "replace the certificate files already in --out"},
			},
		},
		{
			Name:      "revoke",
			Usage:     "Show how to withdraw a signing key from the catalog",
			ArgsUsage: "<signing-public-key-file | key id>",
			Action:    RevokePluginKey,
		},
	},
}

func init() {
	PluginCommand.Commands = append(PluginCommand.Commands, pluginKeyCommand)
}

// InitPluginKeys creates the primary key, a first signing key and its
// certificate. It needs neither settings nor a database.
func InitPluginKeys(_ context.Context, command *cli.Command) error {
	id := command.String("id")
	if !plugin.IsValidID(id) {
		return fmt.Errorf("plugin id %q is not valid", id)
	}
	out := command.String("out")
	names := []string{primaryPublicFile, primarySecretFile, signingPublicFile, signingSecretFile,
		plugin.SignerFileName, plugin.SignerSignatureFileName}
	if err := refuseExisting(out, names, false); err != nil {
		return err
	}

	primaryPublic, primary, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err = writeKeyPair(out, primaryPublicFile, primarySecretFile, primaryPublic, primary, primaryPasswordEnv); err != nil {
		return err
	}
	signing, err := newCertifiedSigningKey(out, id, primary)
	if err != nil {
		return err
	}

	fmt.Printf("primary key %016X: %s and %s\n", primary.ID(), primaryPublicFile, primarySecretFile)
	fmt.Printf("signing key %016X for %s: %s, %s, %s and %s\n", signing.ID(), id,
		signingPublicFile, signingSecretFile, plugin.SignerFileName, plugin.SignerSignatureFileName)
	fmt.Printf(`
Next:
  1. Move %[1]s to offline storage or a password manager and keep a backup.
     It never goes into a repository or CI. Without it no new signing key can
     be certified, and replacing it leaves every older release untrusted.
  2. Put %[2]s into the secrets of your CI and delete the local copy.
  3. Keep %[3]s and %[4]s with the plugin sources, %[5]s packs them into
     every package it signs.
  4. Use the line of %[6]s that starts with RW as author_public_key of the
     catalog entry.
`, filepath.Join(out, primarySecretFile), filepath.Join(out, signingSecretFile),
		plugin.SignerFileName, plugin.SignerSignatureFileName, "nginx-ui plugin pack --key", filepath.Join(out, primaryPublicFile))
	return nil
}

// RotatePluginKey creates a new signing key certified by the primary key.
func RotatePluginKey(_ context.Context, command *cli.Command) error {
	id := command.String("id")
	if !plugin.IsValidID(id) {
		return fmt.Errorf("plugin id %q is not valid", id)
	}
	primary, err := loadKey(command.String("primary"), primaryPasswordEnv)
	if err != nil {
		return err
	}
	out := command.String("out")
	names := []string{signingPublicFile, signingSecretFile, plugin.SignerFileName, plugin.SignerSignatureFileName}
	if err = refuseExisting(out, names, command.Bool("force")); err != nil {
		return err
	}
	signing, err := newCertifiedSigningKey(out, id, primary)
	if err != nil {
		return err
	}
	fmt.Printf("signing key %016X for %s, certified by primary key %016X: %s, %s, %s and %s\n",
		signing.ID(), id, primary.ID(), signingPublicFile, signingSecretFile, plugin.SignerFileName, plugin.SignerSignatureFileName)
	fmt.Println("Replace the signing key in the secrets of your CI and the certificate files next to the plugin sources.")
	return nil
}

// CertifyPluginKey writes a certificate for an existing signing key.
func CertifyPluginKey(_ context.Context, command *cli.Command) error {
	keyPath := command.Args().Get(0)
	if keyPath == "" {
		return errors.New("usage: nginx-ui plugin key certify <signing-public-key-file> --id <plugin-id> --primary <primary.key>")
	}
	signingKey, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}
	primary, err := loadKey(command.String("primary"), primaryPasswordEnv)
	if err != nil {
		return err
	}
	out := command.String("out")
	if err = refuseExisting(out, []string{plugin.SignerFileName, plugin.SignerSignatureFileName}, command.Bool("force")); err != nil {
		return err
	}
	id := command.String("id")
	signer, signature, err := plugin.NewSignerCertificate(signingKey, id, primary)
	if err != nil {
		return err
	}
	if err = writeCertificate(out, signer, signature); err != nil {
		return err
	}
	_, key, err := plugin.InspectSignerCertificate(signer, signature)
	if err != nil {
		return err
	}
	fmt.Printf("certified signing key %s for %s with primary key %016X, wrote %s and %s into %s\n",
		key.KeyID, id, primary.ID(), plugin.SignerFileName, plugin.SignerSignatureFileName, out)
	return nil
}

// RevokePluginKey prints what withdraws a signing key from the catalog. The
// catalog entry is changed through a pull request, so this command only
// names the key id and the place it goes.
func RevokePluginKey(_ context.Context, command *cli.Command) error {
	argument := command.Args().Get(0)
	if argument == "" {
		return errors.New("usage: nginx-ui plugin key revoke <signing-public-key-file | key id>")
	}
	keyID := strings.ToUpper(argument)
	if !keyIDPattern.MatchString(argument) {
		text, err := os.ReadFile(argument)
		if err != nil {
			return err
		}
		var key minisign.PublicKey
		if err = key.UnmarshalText([]byte(strings.TrimSpace(string(text)))); err != nil {
			return fmt.Errorf("%s: %w", argument, err)
		}
		keyID = fmt.Sprintf("%016X", key.ID())
	}
	fmt.Printf(`To withdraw signing key %[1]s, add it to the catalog entry of each plugin
it signed and open a pull request:

  "revoked_signers": ["%[1]s"]

Nginx UI then stops trusting the packages it signed, and the catalog marks the
releases it signed as yanked. Certify a new signing key with
"nginx-ui plugin key rotate" and publish a new release signed with it.
`, keyID)
	return nil
}

// newCertifiedSigningKey creates a signing key in out and certifies it for id.
func newCertifiedSigningKey(out, id string, primary minisign.PrivateKey) (minisign.PrivateKey, error) {
	public, signing, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		return minisign.PrivateKey{}, err
	}
	encoded, err := public.MarshalText()
	if err != nil {
		return minisign.PrivateKey{}, err
	}
	signer, signature, err := plugin.NewSignerCertificate(encoded, id, primary)
	if err != nil {
		return minisign.PrivateKey{}, err
	}
	if err = writeKeyPair(out, signingPublicFile, signingSecretFile, public, signing, signPasswordEnv); err != nil {
		return minisign.PrivateKey{}, err
	}
	return signing, writeCertificate(out, signer, signature)
}

// writeKeyPair writes a minisign key pair. The secret key is encrypted with
// the password in passwordEnv when it is set, and readable by its owner only.
func writeKeyPair(out, publicName, secretName string, public minisign.PublicKey, private minisign.PrivateKey, passwordEnv string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	encodedPublic, err := public.MarshalText()
	if err != nil {
		return err
	}
	var encodedPrivate []byte
	if password := os.Getenv(passwordEnv); password != "" {
		encodedPrivate, err = minisign.EncryptKey(password, private)
	} else {
		encodedPrivate, err = private.MarshalText()
	}
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(out, publicName), append(encodedPublic, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, secretName), append(encodedPrivate, '\n'), 0o600)
}

func writeCertificate(out string, signer, signature []byte) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, plugin.SignerFileName), signer, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, plugin.SignerSignatureFileName), signature, 0o644)
}

// refuseExisting fails when one of names already exists in dir, unless force.
func refuseExisting(dir string, names []string, force bool) error {
	if force {
		return nil
	}
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return fmt.Errorf("%s already exists, choose another --out directory", filepath.Join(dir, name))
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

// loadKey reads a minisign secret key file, decrypting an encrypted one with
// the password in passwordEnv.
func loadKey(path, passwordEnv string) (minisign.PrivateKey, error) {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return minisign.PrivateKey{}, err
	}
	if minisign.IsEncrypted(encoded) {
		key, err := minisign.PrivateKeyFromFile(os.Getenv(passwordEnv), path)
		if err != nil {
			return minisign.PrivateKey{}, fmt.Errorf("decrypt %s with the password in %s: %w", path, passwordEnv, err)
		}
		return key, nil
	}
	var key minisign.PrivateKey
	if err = key.UnmarshalText(encoded); err != nil {
		return minisign.PrivateKey{}, fmt.Errorf("read %s: %w", path, err)
	}
	return key, nil
}

// signerCertificateFiles finds the signer certificate a package of pluginID
// signed with key carries. The files in packageDir come first; without them
// the ones in fallbackDir are returned to be added. It refuses a certificate
// for another plugin or another key, which is what signing with the primary
// key instead of the signing key looks like. found is false without one.
func signerCertificateFiles(packageDir, fallbackDir, pluginID string, key minisign.PrivateKey) (extra map[string][]byte, found bool, err error) {
	signer, signature, err := readCertificatePair(packageDir)
	if err != nil {
		return nil, false, err
	}
	if signer == nil && fallbackDir != "" {
		if signer, signature, err = readCertificatePair(fallbackDir); err != nil {
			return nil, false, err
		}
		if signer != nil {
			extra = map[string][]byte{plugin.SignerFileName: signer, plugin.SignerSignatureFileName: signature}
		}
	}
	if signer == nil {
		return nil, false, nil
	}

	id, certified, err := plugin.InspectSignerCertificate(signer, signature)
	if err != nil {
		return nil, false, err
	}
	if id != pluginID {
		return nil, false, fmt.Errorf("the signer certificate is for %s, the package is %s", id, pluginID)
	}
	if signingID := fmt.Sprintf("%016X", key.ID()); certified.KeyID != signingID {
		return nil, false, fmt.Errorf("the signer certificate certifies key %s, but the package would be signed with key %s; sign with the signing key, not the primary key", certified.KeyID, signingID)
	}
	return extra, true, nil
}

// readCertificatePair reads both certificate files of dir, nil when neither
// exists.
func readCertificatePair(dir string) (signer, signature []byte, err error) {
	read := func(name string) ([]byte, error) {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return body, err
	}
	if signer, err = read(plugin.SignerFileName); err != nil {
		return nil, nil, err
	}
	if signature, err = read(plugin.SignerSignatureFileName); err != nil {
		return nil, nil, err
	}
	if (signer == nil) != (signature == nil) {
		return nil, nil, fmt.Errorf("%s holds only one of %s and %s", dir, plugin.SignerFileName, plugin.SignerSignatureFileName)
	}
	return signer, signature, nil
}

// warnWithoutCertificate notes that a package signed without a certificate
// is listed by the official catalog only when an official or partner key
// signed it.
func warnWithoutCertificate(keyID uint64) {
	if isPluginKey(keyID) {
		return
	}
	fmt.Fprintf(os.Stderr, "note: no %s next to the package or the key, so key %016X signs directly. "+
		"The official catalog lists a community package only when a signing key certified by the primary key signs it, "+
		"see nginx-ui plugin key init.\n", plugin.SignerFileName, keyID)
}
