package upgrader

import (
	"errors"

	"github.com/0xJacky/Nginx-UI/internal/pkgsign"
	"github.com/0xJacky/Nginx-UI/internal/releasesign"
)

var trustedMinisignPublicKeys = releasesign.TrustedPublicKeys()

// verifyArchiveSignature delegates to pkgsign and maps its errors onto the
// upgrader error scope so callers and tests keep their existing contract.
func verifyArchiveSignature(archivePath string, signature []byte) (uint64, error) {
	keyID, err := pkgsign.VerifyFile(archivePath, signature, trustedMinisignPublicKeys)
	if err == nil {
		return keyID, nil
	}
	switch {
	case errors.Is(err, pkgsign.ErrSignatureInvalid):
		return keyID, ErrSignatureInvalid
	case errors.Is(err, pkgsign.ErrSignatureKeyUnknown):
		return keyID, ErrSignatureKeyUnknown
	case errors.Is(err, pkgsign.ErrTrustedSignatureKeysEmpty):
		return keyID, ErrTrustedSignatureKeysEmpty
	case errors.Is(err, pkgsign.ErrTrustedSignatureKeysBad):
		return keyID, ErrTrustedSignatureKeysInvalid
	}
	return keyID, err
}
