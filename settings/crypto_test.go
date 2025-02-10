package settings

import (
	"crypto/md5"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSecretMd5_WithNonEmptySecret_ReturnsExpectedMd5Hash(t *testing.T) {
	// Init
	CryptoSettings.Secret = "testSecret"
	expectedMd5 := md5.Sum([]byte("testSecret"))
	expectedMd5String := hex.EncodeToString(expectedMd5[:])

	// Execute
	resultMd5 := CryptoSettings.GetSecretMd5()
	resultMd5String := hex.EncodeToString(resultMd5[:])

	// Verify
	assert.Equal(t, expectedMd5String, resultMd5String, "MD5 hash should match for non-empty secret")
}

func TestGetSecretMd5_WithEmptySecret_ReturnsMd5OfEmptyString(t *testing.T) {
	// Init
	CryptoSettings.Secret = ""
	expectedMd5 := md5.Sum([]byte(""))
	expectedMd5String := hex.EncodeToString(expectedMd5[:])

	// Execute
	resultMd5 := CryptoSettings.GetSecretMd5()
	resultMd5String := hex.EncodeToString(resultMd5[:])

	// Verify
	assert.Equal(t, expectedMd5String, resultMd5String, "MD5 hash of an empty string should be returned for empty secret")
}

func TestGetSecretMd5_WithDifferentSecrets_ReturnsDifferentMd5Hashes(t *testing.T) {
	// Init
	CryptoSettings.Secret = "secret1"
	firstMd5 := CryptoSettings.GetSecretMd5()
	CryptoSettings.Secret = "secret2"
	secondMd5 := CryptoSettings.GetSecretMd5()

	// Verify
	assert.NotEqual(t, firstMd5, secondMd5, "Different secrets should produce different MD5 hashes")
}
