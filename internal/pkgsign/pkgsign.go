// Package pkgsign verifies minisign signatures of downloaded archives. It is
// shared by the core upgrader and the plugin installer.
package pkgsign

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/internal/releasesign"
)

var (
	ErrSignatureInvalid          = errors.New("archive signature is invalid")
	ErrSignatureKeyUnknown       = errors.New("archive signature key is not trusted")
	ErrTrustedSignatureKeysEmpty = errors.New("no trusted signature keys configured")
	ErrTrustedSignatureKeysBad   = errors.New("trusted signature keys are invalid")
)

// VerifyFile checks signature against the file at path using the given
// trusted public keys (minisign text form). It returns the key id that
// produced the signature.
func VerifyFile(path string, signature []byte, trustedKeys []string) (uint64, error) {
	keyID, publicKey, err := signerKey(signature, trustedKeys)
	if err != nil {
		return keyID, err
	}

	f, err := os.Open(path)
	if err != nil {
		return keyID, err
	}
	defer f.Close()

	reader := minisign.NewReader(f)
	if _, err = io.Copy(io.Discard, reader); err != nil {
		return keyID, err
	}
	if !reader.Verify(publicKey, signature) {
		return keyID, ErrSignatureInvalid
	}
	return keyID, nil
}

// VerifyBytes checks signature against message using the given trusted
// public keys (minisign text form). Both the legacy and the prehashed
// algorithm are accepted. It returns the key id that produced the signature.
func VerifyBytes(message, signature []byte, trustedKeys []string) (uint64, error) {
	keyID, publicKey, err := signerKey(signature, trustedKeys)
	if err != nil {
		return keyID, err
	}
	if !minisign.Verify(publicKey, message, signature) {
		return keyID, ErrSignatureInvalid
	}
	return keyID, nil
}

// KeyID returns the id of the key a minisign signature names.
func KeyID(signature []byte) (uint64, error) {
	var parsed minisign.Signature
	if err := parsed.UnmarshalText(signature); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrSignatureInvalid, err)
	}
	return parsed.KeyID, nil
}

// signerKey finds the trusted key a signature names.
func signerKey(signature []byte, trustedKeys []string) (uint64, minisign.PublicKey, error) {
	keyID, err := KeyID(signature)
	if err != nil {
		return 0, minisign.PublicKey{}, err
	}
	keys, err := ParseTrustedKeys(trustedKeys)
	if err != nil {
		return keyID, minisign.PublicKey{}, err
	}
	publicKey, ok := keys[keyID]
	if !ok {
		return keyID, minisign.PublicKey{}, fmt.Errorf("%w: %016X", ErrSignatureKeyUnknown, keyID)
	}
	return keyID, publicKey, nil
}

// VerifyFileWithReleaseKeys verifies against the release keys pinned in the binary.
func VerifyFileWithReleaseKeys(path string, signature []byte) (uint64, error) {
	return VerifyFile(path, signature, releasesign.TrustedPublicKeys())
}

// ParseTrustedKeys parses minisign public keys and indexes them by key id.
func ParseTrustedKeys(encodedKeys []string) (map[uint64]minisign.PublicKey, error) {
	keys := make(map[uint64]minisign.PublicKey)
	for _, encoded := range encodedKeys {
		encoded = strings.TrimSpace(encoded)
		if encoded == "" {
			continue
		}
		var publicKey minisign.PublicKey
		if err := publicKey.UnmarshalText([]byte(encoded)); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrTrustedSignatureKeysBad, err)
		}
		if _, exists := keys[publicKey.ID()]; exists {
			return nil, fmt.Errorf("%w: duplicate key ID %016X", ErrTrustedSignatureKeysBad, publicKey.ID())
		}
		keys[publicKey.ID()] = publicKey
	}
	if len(keys) == 0 {
		return nil, ErrTrustedSignatureKeysEmpty
	}
	return keys, nil
}
