package upgrader

import (
	"fmt"
	"io"
	"os"
	"strings"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/internal/releasesign"
)

var trustedMinisignPublicKeys = releasesign.TrustedPublicKeys()

func verifyArchiveSignature(archivePath string, signature []byte) (uint64, error) {
	var parsedSignature minisign.Signature
	if err := parsedSignature.UnmarshalText(signature); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrSignatureInvalid, err)
	}

	trustedKeys, err := parseTrustedMinisignKeys(trustedMinisignPublicKeys)
	if err != nil {
		return 0, err
	}
	publicKey, ok := trustedKeys[parsedSignature.KeyID]
	if !ok {
		return parsedSignature.KeyID, fmt.Errorf("%w: %016X", ErrSignatureKeyUnknown, parsedSignature.KeyID)
	}

	archive, err := os.Open(archivePath)
	if err != nil {
		return parsedSignature.KeyID, err
	}
	defer archive.Close()

	reader := minisign.NewReader(archive)
	if _, err = io.Copy(io.Discard, reader); err != nil {
		return parsedSignature.KeyID, err
	}
	if !reader.Verify(publicKey, signature) {
		return parsedSignature.KeyID, ErrSignatureInvalid
	}
	return parsedSignature.KeyID, nil
}

func parseTrustedMinisignKeys(encodedKeys []string) (map[uint64]minisign.PublicKey, error) {
	trustedKeys := make(map[uint64]minisign.PublicKey)
	for _, encodedKey := range encodedKeys {
		encodedKey = strings.TrimSpace(encodedKey)
		if encodedKey == "" {
			continue
		}
		var publicKey minisign.PublicKey
		if err := publicKey.UnmarshalText([]byte(encodedKey)); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrTrustedSignatureKeysInvalid, err)
		}
		if _, exists := trustedKeys[publicKey.ID()]; exists {
			return nil, fmt.Errorf("%w: duplicate key ID %016X", ErrTrustedSignatureKeysInvalid, publicKey.ID())
		}
		trustedKeys[publicKey.ID()] = publicKey
	}
	if len(trustedKeys) == 0 {
		return nil, ErrTrustedSignatureKeysEmpty
	}
	return trustedKeys, nil
}
