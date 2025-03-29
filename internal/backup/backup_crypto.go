package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"

	"github.com/uozi-tech/cosy"
)

// AESEncrypt encrypts data using AES-256-CBC
func AESEncrypt(data []byte, key []byte, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrEncryptData, err.Error())
	}

	// Pad data to be a multiple of block size
	padding := aes.BlockSize - (len(data) % aes.BlockSize)
	padtext := make([]byte, len(data)+padding)
	copy(padtext, data)
	// PKCS#7 padding
	for i := len(data); i < len(padtext); i++ {
		padtext[i] = byte(padding)
	}

	// Create CBC encrypter
	mode := cipher.NewCBCEncrypter(block, iv)
	encrypted := make([]byte, len(padtext))
	mode.CryptBlocks(encrypted, padtext)

	return encrypted, nil
}

// AESDecrypt decrypts data using AES-256-CBC
func AESDecrypt(encrypted []byte, key []byte, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrDecryptData, err.Error())
	}

	// Create CBC decrypter
	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(encrypted))
	mode.CryptBlocks(decrypted, encrypted)

	// Remove padding
	padding := int(decrypted[len(decrypted)-1])
	if padding < 1 || padding > aes.BlockSize {
		return nil, ErrInvalidPadding
	}
	return decrypted[:len(decrypted)-padding], nil
}

// GenerateAESKey generates a random 32-byte AES key
func GenerateAESKey() ([]byte, error) {
	key := make([]byte, 32) // 256-bit key
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, cosy.WrapErrorWithParams(ErrGenerateAESKey, err.Error())
	}
	return key, nil
}

// GenerateIV generates a random 16-byte initialization vector
func GenerateIV() ([]byte, error) {
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, cosy.WrapErrorWithParams(ErrGenerateIV, err.Error())
	}
	return iv, nil
}

// encryptFile encrypts a single file using AES encryption
func encryptFile(filePath string, key []byte, iv []byte) error {
	// Read file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrReadFile, filePath)
	}

	// Encrypt file content
	encrypted, err := AESEncrypt(data, key, iv)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrEncryptFile, filePath)
	}

	// Write encrypted content back
	if err := os.WriteFile(filePath, encrypted, 0644); err != nil {
		return cosy.WrapErrorWithParams(ErrWriteEncryptedFile, filePath)
	}

	return nil
}

// decryptFile decrypts a single file using AES decryption
func decryptFile(filePath string, key []byte, iv []byte) error {
	// Read encrypted file content
	encryptedData, err := os.ReadFile(filePath)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrReadEncryptedFile, err.Error())
	}

	// Decrypt file content
	decryptedData, err := AESDecrypt(encryptedData, key, iv)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrDecryptFile, err.Error())
	}

	// Write decrypted content back
	if err := os.WriteFile(filePath, decryptedData, 0644); err != nil {
		return cosy.WrapErrorWithParams(ErrWriteDecryptedFile, err.Error())
	}

	return nil
}

// EncodeToBase64 encodes byte slice to base64 string
func EncodeToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeFromBase64 decodes base64 string to byte slice
func DecodeFromBase64(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}
