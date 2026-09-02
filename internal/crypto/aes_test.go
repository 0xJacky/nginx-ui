package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"

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

	block, err := aes.NewCipher(settings.CryptoSettings.GetSecretMd5())
	require.NoError(t, err)

	// AesDecrypt decrypts the body and then base64-decodes it, so the body has to
	// decrypt to something base64 rejects. Corrupting one byte of a real ciphertext
	// only lands on invalid base64 by chance, which made this test flaky.
	payload := []byte("not valid base64!!")
	input := make([]byte, aes.BlockSize+len(payload))
	iv := input[:aes.BlockSize]
	cipher.NewCFBEncrypter(block, iv).XORKeyStream(input[aes.BlockSize:], payload)

	_, err = AesDecrypt(input)
	require.Error(t, err, "decrypting a payload that is not base64 should return an error")
}
