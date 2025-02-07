package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/uozi-tech/cosy/logger"
)

const (
	CacheKey = "crypto"
	timeout  = 10 * time.Minute
)

type Params struct {
	PrivateKey string `json:"-"`
	PublicKey  string `json:"public_key"`
}

// GenerateRSAKeyPair generates a new RSA key pair
func GenerateRSAKeyPair() (privateKeyPEM, publicKeyPEM []byte, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	privateKeyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	publicKeyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&privateKey.PublicKey),
	})
	return
}

// GetCryptoParams registers a new key pair in the cache if it doesn't exist
// otherwise, it returns the existing nonce and public key
func GetCryptoParams() (params *Params, err error) {
	// Check if the key pair exists in then cache
	if value, ok := cache.Get(CacheKey); ok {
		return value.(*Params), nil
	}
	// Generate a nonce = hash(publicKey)
	privateKeyPEM, publicKeyPEM, err := GenerateRSAKeyPair()
	if err != nil {
		return nil, err
	}
	params = &Params{
		PrivateKey: string(privateKeyPEM),
		PublicKey:  string(publicKeyPEM),
	}
	cache.Set(CacheKey, params, timeout)
	return
}

// Decrypt decrypts the data with the private key (nonce, paramEncrypted)
func Decrypt(paramEncrypted string) (data map[string]interface{}, err error) {
	// Get crypto params from cache
	value, ok := cache.Get(CacheKey)
	if !ok {
		return nil, ErrTimeout
	}

	params := value.(*Params)
	block, _ := pem.Decode([]byte(params.PrivateKey))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		logger.Errorf("failed to parse private key: %v", err)
		return nil, err
	}

	paramEncryptedDecoded, err := base64.StdEncoding.DecodeString(paramEncrypted)
	if err != nil {
		logger.Errorf("base64 decode error: %v", err)
		return nil, err
	}

	decrypted, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, paramEncryptedDecoded)
	if err != nil {
		logger.Errorf("decryption failed: %v", err)
		return nil, err
	}

	err = json.Unmarshal(decrypted, &data)
	if err != nil {
		return nil, err
	}

	return
}
