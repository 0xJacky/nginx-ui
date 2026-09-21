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
	var parsed minisign.Signature
	if err := parsed.UnmarshalText(signature); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrSignatureInvalid, err)
	}

	keys, err := ParseTrustedKeys(trustedKeys)
	if err != nil {
		return 0, err
	}
	publicKey, ok := keys[parsed.KeyID]
	if !ok {
		return parsed.KeyID, fmt.Errorf("%w: %016X", ErrSignatureKeyUnknown, parsed.KeyID)
	}

	f, err := os.Open(path)
	if err != nil {
		return parsed.KeyID, err
	}
	defer f.Close()

	reader := minisign.NewReader(f)
	if _, err = io.Copy(io.Discard, reader); err != nil {
		return parsed.KeyID, err
	}
	if !reader.Verify(publicKey, signature) {
		return parsed.KeyID, ErrSignatureInvalid
	}
	return parsed.KeyID, nil
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
