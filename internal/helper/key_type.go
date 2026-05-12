package helper

import "github.com/go-acme/lego/v5/certcrypto"

const (
	legacyEC256   certcrypto.KeyType = "P256"
	legacyEC384   certcrypto.KeyType = "P384"
	legacyRSA2048 certcrypto.KeyType = "2048"
	legacyRSA3072 certcrypto.KeyType = "3072"
	legacyRSA4096 certcrypto.KeyType = "4096"
	legacyRSA8192 certcrypto.KeyType = "8192"
)

func GetKeyType(keyType certcrypto.KeyType) certcrypto.KeyType {
	switch keyType {
	case certcrypto.RSA2048, certcrypto.RSA3072, certcrypto.RSA4096, certcrypto.RSA8192,
		certcrypto.EC256, certcrypto.EC384:
		return keyType
	case legacyEC256:
		return certcrypto.EC256
	case legacyEC384:
		return certcrypto.EC384
	case legacyRSA2048:
		return certcrypto.RSA2048
	case legacyRSA3072:
		return certcrypto.RSA3072
	case legacyRSA4096:
		return certcrypto.RSA4096
	case legacyRSA8192:
		return certcrypto.RSA8192
	}
	return certcrypto.RSA2048
}

func GetKeyTypeAliases(keyType certcrypto.KeyType) []certcrypto.KeyType {
	normalizedKeyType := GetKeyType(keyType)

	switch normalizedKeyType {
	case certcrypto.EC256:
		return []certcrypto.KeyType{certcrypto.EC256, legacyEC256}
	case certcrypto.EC384:
		return []certcrypto.KeyType{certcrypto.EC384, legacyEC384}
	case certcrypto.RSA2048:
		return []certcrypto.KeyType{certcrypto.RSA2048, legacyRSA2048}
	case certcrypto.RSA3072:
		return []certcrypto.KeyType{certcrypto.RSA3072, legacyRSA3072}
	case certcrypto.RSA4096:
		return []certcrypto.KeyType{certcrypto.RSA4096, legacyRSA4096}
	case certcrypto.RSA8192:
		return []certcrypto.KeyType{certcrypto.RSA8192, legacyRSA8192}
	}

	return []certcrypto.KeyType{normalizedKeyType}
}

func GetKeyTypeAliasStrings(keyType certcrypto.KeyType) []string {
	aliases := GetKeyTypeAliases(keyType)
	values := make([]string, 0, len(aliases))

	for _, alias := range aliases {
		values = append(values, string(alias))
	}

	return values
}

func IsValidKeyType(keyType certcrypto.KeyType) bool {
	switch keyType {
	case "", certcrypto.RSA2048, certcrypto.RSA3072, certcrypto.RSA4096, certcrypto.RSA8192,
		certcrypto.EC256, certcrypto.EC384, legacyEC256, legacyEC384,
		legacyRSA2048, legacyRSA3072, legacyRSA4096, legacyRSA8192:
		return true
	}
	return false
}
