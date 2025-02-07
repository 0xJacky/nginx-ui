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
	CacheKey = "sign"
	timeout  = 10 * time.Minute
)

type Sign struct {
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
func GetCryptoParams() (sign *Sign, err error) {
	// Check if the key pair exists in then cache
	if sign, ok := cache.Get(CacheKey); ok {
		return sign.(*Sign), nil
	}
	// Generate a nonce = hash(publicKey)
	privateKeyPEM, publicKeyPEM, err := GenerateRSAKeyPair()
	if err != nil {
		return nil, err
	}
	sign = &Sign{
		PrivateKey: string(privateKeyPEM),
		PublicKey:  string(publicKeyPEM),
	}
	cache.Set(CacheKey, sign, timeout)
	return
}

// Decrypt decrypts the data with the private key (nonce, paramEncrypted)
func Decrypt(paramEncrypted string) (data map[string]interface{}, err error) {
	// Get sign params from cache
	sign, ok := cache.Get(CacheKey)
	if !ok {
		return nil, ErrTimeout
	}

	signParams := sign.(*Sign)
	block, _ := pem.Decode([]byte(signParams.PrivateKey))
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
