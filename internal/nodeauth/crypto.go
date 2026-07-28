package nodeauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
)

const credentialEnvelopeVersion byte = 1

func EncryptPrivateCredential(purpose string, plaintext []byte) ([]byte, error) {
	return encryptCredential(purpose, plaintext)
}

func DecryptPrivateCredential(purpose string, envelope []byte) ([]byte, error) {
	return decryptCredential(purpose, envelope)
}

func LegacyCredentialPurpose(nodeID uint64) string {
	return fmt.Sprintf("node-legacy-secret:%d", nodeID)
}

func SigningCredentialPurpose(credentialID string) string {
	return "node-signing-private-key:" + credentialID
}

func encryptCredential(purpose string, plaintext []byte) ([]byte, error) {
	aead, err := credentialAEAD()
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate credential nonce: %w", err)
	}

	header := []byte{credentialEnvelopeVersion, byte(len(nonce))}
	ciphertext := aead.Seal(nil, nonce, plaintext, credentialAAD(purpose, header))
	envelope := make([]byte, 0, len(header)+len(nonce)+len(ciphertext))
	envelope = append(envelope, header...)
	envelope = append(envelope, nonce...)
	envelope = append(envelope, ciphertext...)
	return envelope, nil
}

func decryptCredential(purpose string, envelope []byte) ([]byte, error) {
	if len(envelope) < 2 || envelope[0] != credentialEnvelopeVersion {
		return nil, errors.New("unsupported node credential envelope")
	}
	nonceSize := int(envelope[1])
	if nonceSize <= 0 || len(envelope) <= 2+nonceSize {
		return nil, errors.New("invalid node credential envelope")
	}

	aead, err := credentialAEAD()
	if err != nil {
		return nil, err
	}
	if nonceSize != aead.NonceSize() {
		return nil, errors.New("invalid node credential nonce size")
	}
	header := envelope[:2]
	nonce := envelope[2 : 2+nonceSize]
	ciphertext := envelope[2+nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, credentialAAD(purpose, header))
	if err != nil {
		return nil, errors.New("node credential authentication failed")
	}
	return plaintext, nil
}

func credentialAEAD() (cipher.AEAD, error) {
	secret := strings.TrimSpace(settings.CryptoSettings.Secret)
	instanceID := strings.TrimSpace(settings.NodeSettings.InstanceID)
	if secret == "" || instanceID == "" {
		return nil, errors.New("node credential root key is unavailable")
	}

	key, err := hkdf.Key(sha256.New, []byte(secret), []byte(instanceID), "nginx-ui/node-credential/aes-256-gcm/v1", 32)
	if err != nil {
		return nil, fmt.Errorf("derive node credential key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func credentialAAD(purpose string, header []byte) []byte {
	instanceID := settings.NodeSettings.InstanceID
	buffer := make([]byte, 0, len(header)+8+len(instanceID)+len(purpose))
	buffer = append(buffer, header...)
	lengths := make([]byte, 8)
	binary.BigEndian.PutUint32(lengths[:4], uint32(len(instanceID)))
	binary.BigEndian.PutUint32(lengths[4:], uint32(len(purpose)))
	buffer = append(buffer, lengths...)
	buffer = append(buffer, instanceID...)
	buffer = append(buffer, purpose...)
	return buffer
}
