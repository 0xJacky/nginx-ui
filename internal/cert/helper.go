package cert

import (
	"crypto/x509"
	"encoding/pem"
	"os"
)

func IsCertificate(pemStr string) bool {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return false
	}
	_, err := x509.ParseCertificate(block.Bytes)
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
	if errECDSA == nil {
		return true
	}

	_, errPKC := x509.ParsePKCS8PrivateKey(block.Bytes)
	return errPKC == nil
}

// IsCertificatePath checks if the file at the given path is a certificate or not exists.
func IsCertificatePath(path string) bool {
	if path == "" {
		return false
	}
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

	return IsCertificate(string(bytes))
}

// IsPrivateKeyPath checks if the file at the given path is a private key or not exists.
func IsPrivateKeyPath(path string) bool {
	if path == "" {
		return false
	}

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
