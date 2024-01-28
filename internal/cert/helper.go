package cert

import (
	"crypto/x509"
	"encoding/pem"
	"os"
)

func IsPublicKey(pemStr string) bool {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return false
	}

	_, err := x509.ParsePKIXPublicKey(block.Bytes)
	return err == nil
}

func IsPrivateKey(pemStr string) bool {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return false
	}

	_, errRSA := x509.ParsePKCS1PrivateKey(block.Bytes)
	if errRSA == nil {
		return true
	}

	_, errECDSA := x509.ParseECPrivateKey(block.Bytes)
	return errECDSA == nil
}

// IsPublicKeyPath checks if the file at the given path is a public key or not exists.
func IsPublicKeyPath(path string) bool {
	_, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			return true
		}
		return false
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	return IsPublicKey(string(bytes))
}

// IsPrivateKeyPath checks if the file at the given path is a private key or not exists.
func IsPrivateKeyPath(path string) bool {
	_, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			return true
		}
		return false
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	return IsPrivateKey(string(bytes))
}
