package plugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/internal/pkgsign"
	"github.com/uozi-tech/cosy/logger"
)

// A signer certificate lets an author sign packages with a signing key while
// the catalog lists only the author's primary key. It is two ordinary files at
// the package root: plugin.signer holds the signing public key,
// plugin.signer.minisig is a primary key signature over it whose trusted
// comment names the plugin the key may sign.
const (
	SignerFileName          = "plugin.signer"
	SignerSignatureFileName = "plugin.signer.minisig"
)

// signerCommentPrefix starts the trusted comment of a signer certificate.
const signerCommentPrefix = "signer:"

// Certificate problems the linter reports under their own rules.
var (
	errSignerIncomplete = errors.New("signer certificate is incomplete")
	errSignerPlugin     = errors.New("signer certificate names another plugin")
	errSignerRevoked    = errors.New("signing key is revoked")
)

// signerCertificate is a signing key a primary key certified for one plugin.
type signerCertificate struct {
	PluginID string
	// KeyID is the signing key id in upper case hex.
	KeyID string
	// Key is the signing public key in its two line text form.
	Key string
	// Primary is the primary public key that issued the certificate, as the
	// caller passed it.
	Primary string
}

// readSignerCertificate reads the signer certificate of an extracted package
// and checks it against the primary keys. It returns nil without one. The
// certificate must name the plugin of the package and its key must not be
// revoked.
func readSignerCertificate(root string, primaries, revoked []string) (*signerCertificate, error) {
	signer, signature, err := readSignerFiles(root)
	if signer == nil || err != nil {
		return nil, err
	}
	issuer := ""
	for _, primary := range primaries {
		if _, err = pkgsign.VerifyBytes(signer, signature, []string{primary}); err == nil {
			issuer = primary
			break
		}
	}
	if issuer == "" {
		return nil, fmt.Errorf("%s does not verify with the author key", SignerSignatureFileName)
	}
	certificate, err := parseSignerCertificate(root, signer, signature)
	if err != nil {
		return nil, err
	}
	certificate.Primary = issuer
	if slices.ContainsFunc(revoked, func(keyID string) bool { return strings.EqualFold(keyID, certificate.KeyID) }) {
		return nil, fmt.Errorf("%w: %s", errSignerRevoked, certificate.KeyID)
	}
	return certificate, nil
}

// readSignerFiles reads the two certificate files, both nil without a
// certificate and an error when only one of them is there.
func readSignerFiles(root string) (signer, signature []byte, err error) {
	if signer, err = readRootFile(root, SignerFileName); err != nil {
		return nil, nil, err
	}
	if signature, err = readRootFile(root, SignerSignatureFileName); err != nil {
		return nil, nil, err
	}
	switch {
	case signer == nil && signature == nil:
		return nil, nil, nil
	case signer == nil:
		return nil, nil, fmt.Errorf("%w: %s is present without %s", errSignerIncomplete, SignerSignatureFileName, SignerFileName)
	case signature == nil:
		return nil, nil, fmt.Errorf("%w: %s is present without %s", errSignerIncomplete, SignerFileName, SignerSignatureFileName)
	}
	return signer, signature, nil
}

// parseSignerCertificate reads the signing key and the plugin id of a
// certificate whose issuer is checked elsewhere, and requires the id to be
// the one of the package at root. The linter, which knows no primary key,
// stops here.
func parseSignerCertificate(root string, signer, signature []byte) (*signerCertificate, error) {
	var parsed minisign.Signature
	if err := parsed.UnmarshalText(signature); err != nil {
		return nil, err
	}
	id, ok := strings.CutPrefix(parsed.TrustedComment, signerCommentPrefix)
	if !ok || !IsValidID(id) {
		return nil, fmt.Errorf("trusted comment %q is not signer:<plugin id>", parsed.TrustedComment)
	}
	key, err := parsePartnerKey(signer)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", SignerFileName, err)
	}
	packageID, err := packagePluginID(root)
	if err != nil {
		return nil, err
	}
	if id != packageID {
		return nil, fmt.Errorf("%w: it is for %s, the package is %s", errSignerPlugin, id, packageID)
	}
	return &signerCertificate{PluginID: id, KeyID: key.KeyID, Key: key.Key}, nil
}

// packageSigner is the signer certificate of a package that holds up, nil
// otherwise. A certificate that does not is ignored with a debug log, so the
// package is only trusted for a signature a primary key made itself.
func packageSigner(root string, primaries, revoked []string) *signerCertificate {
	if len(primaries) == 0 {
		return nil
	}
	certificate, err := readSignerCertificate(root, primaries, revoked)
	if err != nil {
		logger.Debugf("Ignore the plugin signer certificate: %v", err)
		return nil
	}
	return certificate
}

// packagePluginID reads the id of plugin.json at the package root.
func packagePluginID(root string) (string, error) {
	raw, err := readRootFile(root, ManifestFileName)
	if err != nil {
		return "", err
	}
	if raw == nil {
		return "", fmt.Errorf("%s is missing", ManifestFileName)
	}
	var manifest struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		return "", fmt.Errorf("%s: %w", ManifestFileName, err)
	}
	return manifest.ID, nil
}

// NewSignerCertificate certifies a signing key for one plugin with a primary
// key. It returns the contents of plugin.signer, the signing key in its two
// line form, and of plugin.signer.minisig, whose trusted comment names the
// plugin.
func NewSignerCertificate(signingKey []byte, pluginID string, primary minisign.PrivateKey) (signer, signature []byte, err error) {
	key, err := parsePartnerKey(signingKey)
	if err != nil {
		return nil, nil, fmt.Errorf("signing key: %w", err)
	}
	if !IsValidID(pluginID) {
		return nil, nil, fmt.Errorf("plugin id %q is not valid", pluginID)
	}
	if key.KeyID == fmt.Sprintf("%016X", primary.ID()) {
		return nil, nil, errors.New("the signing key is the primary key itself")
	}

	signer = []byte(key.Key + "\n")
	reader := minisign.NewReader(bytes.NewReader(signer))
	if _, err = io.Copy(io.Discard, reader); err != nil {
		return nil, nil, err
	}
	untrusted := fmt.Sprintf("signer certificate of %s, key %s", pluginID, key.KeyID)
	return signer, reader.SignWithComments(primary, signerCommentPrefix+pluginID, untrusted), nil
}
