package cert

import (
	"crypto/ecdsa"
	"crypto/rsa"
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

// GetKeyType determines the key type from a PEM certificate string.
// Returns "2048", "3072", "4096", "P256", "P384" or empty string.
func GetKeyType(pemStr string) (string, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return "", ErrCertDecode
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", ErrCertParse
	}

	switch cert.PublicKeyAlgorithm {
	case x509.RSA:
		rsaKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return "", nil
		}
		keySize := rsaKey.Size() * 8 // Size returns size in bytes, convert to bits
		switch keySize {
		case 2048:
			return "2048", nil
		case 3072:
			return "3072", nil
		case 4096:
			return "4096", nil
		default:
			return "", nil
		}
	case x509.ECDSA:
		ecKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
		if !ok {
			return "", nil
		}
		curve := ecKey.Curve.Params().Name
		switch curve {
		case "P-256":
			return "P256", nil
		case "P-384":
			return "P384", nil
		default:
			return "", nil
		}
	default:
		return "", nil
	}
}

// GetKeyTypeFromPath determines the key type from a certificate file.
// Returns "2048", "3072", "4096", "P256", "P384" or empty string.
func GetKeyTypeFromPath(path string) (string, error) {
	if path == "" {
		return "", ErrCertPathIsEmpty
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return GetKeyType(string(bytes))
}
