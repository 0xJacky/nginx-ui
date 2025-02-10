package crypto

import (
	"github.com/0xJacky/Nginx-UI/settings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func EncryptDecryptRoundTrip(text string) bool {
	encrypted, err := AesEncrypt([]byte(text))
	if err != nil {
		return false
	}

	decrypted, err := AesDecrypt(encrypted)
	if err != nil {
		return false
	}

	return text == string(decrypted)
}

func EncryptsNonEmptyStringWithoutError(text string) bool {
	_, err := AesEncrypt([]byte(text))
	return err == nil
}

func DecryptsToOriginalTextAfterEncryption(text string) bool {
	encrypted, _ := AesEncrypt([]byte(text))
	decrypted, err := AesDecrypt(encrypted)
	if err != nil {
		return false
	}

	return text == string(decrypted)
}

func FailsToDecryptWithModifiedCiphertext(text string) bool {
	encrypted, _ := AesEncrypt([]byte(text))
	// Modify the ciphertext
	encrypted[0] ^= 0xff
	_, err := AesDecrypt(encrypted)
	return err != nil
}

func FailsToDecryptShortCiphertext() bool {
	_, err := AesDecrypt([]byte("short"))
	return err != nil
}

func TestAesEncryptionDecryption(t *testing.T) {
	settings.CryptoSettings.Secret = "test"
	assert.True(t, EncryptDecryptRoundTrip("Hello, world!"), "should encrypt and decrypt to the original text")
	assert.True(t, EncryptsNonEmptyStringWithoutError("Test String"), "should encrypt a non-empty string without error")
	assert.True(t, DecryptsToOriginalTextAfterEncryption("Another Test String"), "should decrypt to the original text after encryption")
	assert.True(t, FailsToDecryptWithModifiedCiphertext("Sensitive Data"), "should fail to decrypt with modified ciphertext")
	assert.True(t, FailsToDecryptShortCiphertext(), "should fail to decrypt short ciphertext")
}

func TestAesEncrypt_WithEmptyString_ReturnsError(t *testing.T) {
	settings.CryptoSettings.Secret = "test"
	_, err := AesEncrypt([]byte(""))
	require.Error(t, err, "encrypting an empty string should return an error")
}

func TestAesDecrypt_WithInvalidBase64_ReturnsError(t *testing.T) {
	settings.CryptoSettings.Secret = "test"
	// Assuming the function is modified to handle this case explicitly
	encrypted, _ := AesEncrypt([]byte("valid text"))
	// Invalidate the base64 encoding
	encrypted[len(encrypted)-1] = '!'
	_, err := AesDecrypt(encrypted)
	require.Error(t, err, "decrypting an invalid base64 string should return an error")
}
