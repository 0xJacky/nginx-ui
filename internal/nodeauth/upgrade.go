package nodeauth

import (
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
)

// The upgrade request itself is authenticated by the ordinary request signature,
// which already proves knowledge of the shared secret and carries its own nonce
// and replay window. What the transport cannot provide is the other direction:
// the controller discards nothing until it knows it reached the node that holds
// that secret, so the target signs its answer with the same key. Without it, an
// attacker able to rewrite a plaintext response could hand back a credential ID
// the target never issued and leave the relationship permanently broken.
const (
	upgradeConfirmationDomain = "nginx-ui-legacy-upgrade-response-v1"
	upgradeKeyInfo            = "nginx-ui/legacy-upgrade/hmac-sha256/v1"
)

var ErrUpgradeProofInvalid = errors.New("legacy upgrade confirmation is invalid")

// SignUpgradeConfirmation authenticates the credential a target issued for the
// public key the controller just sent. That key is freshly generated per
// upgrade, so it doubles as the value binding this answer to this exchange.
func SignUpgradeConfirmation(secret []byte, controllerInstanceID string, publicKey []byte,
	credentialID, targetInstanceID string,
) (string, error) {
	if !validIdentifier(controllerInstanceID) {
		return "", errors.New("invalid legacy upgrade controller instance ID")
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return "", errors.New("invalid legacy upgrade public key")
	}
	if !validIdentifier(credentialID) || !validIdentifier(targetInstanceID) {
		return "", errors.New("invalid legacy upgrade confirmation identifier")
	}
	mac, err := upgradeMAC(secret, controllerInstanceID,
		publicKey,
		[]byte(credentialID),
		[]byte(targetInstanceID),
	)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(mac), nil
}

// VerifyUpgradeConfirmation checks the target's answer before the controller
// starts relying on the credential it describes.
func VerifyUpgradeConfirmation(secret []byte, controllerInstanceID string, publicKey []byte,
	credentialID, targetInstanceID, confirmation string,
) error {
	expected, err := SignUpgradeConfirmation(secret, controllerInstanceID, publicKey, credentialID, targetInstanceID)
	if err != nil {
		return err
	}
	if len(confirmation) != len(expected) ||
		subtle.ConstantTimeCompare([]byte(confirmation), []byte(expected)) != 1 {
		return ErrUpgradeProofInvalid
	}
	return nil
}

// upgradeMAC derives a key bound to the controller identity through the HKDF
// salt, then authenticates the length-prefixed parts so no concatenation of
// them can be reinterpreted as a different message.
func upgradeMAC(secret []byte, controllerInstanceID string, parts ...[]byte) ([]byte, error) {
	if len(secret) == 0 {
		return nil, errors.New("legacy node secret is unavailable")
	}
	key, err := hkdf.Key(sha256.New, secret, []byte(controllerInstanceID), upgradeKeyInfo, 32)
	if err != nil {
		return nil, fmt.Errorf("derive legacy upgrade key: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(upgradeConfirmationDomain))
	length := make([]byte, 4)
	for _, part := range parts {
		binary.BigEndian.PutUint32(length, uint32(len(part)))
		_, _ = mac.Write(length)
		_, _ = mac.Write(part)
	}
	return mac.Sum(nil), nil
}
