package helper

import "github.com/go-acme/lego/v4/certcrypto"

func GetKeyType(keyType certcrypto.KeyType) certcrypto.KeyType {
	switch keyType {
	case certcrypto.RSA2048, certcrypto.RSA3072, certcrypto.RSA4096, certcrypto.RSA8192,
		certcrypto.EC256, certcrypto.EC384:
		return keyType
	}
	return certcrypto.RSA2048
}
