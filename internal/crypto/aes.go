package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/pkg/errors"
	"io"
)

// AesEncrypt encrypts text and given key with AES.
func AesEncrypt(text []byte) ([]byte, error) {
	if len(text) == 0 {
		return nil, errors.New("AesEncrypt text is empty")
	}
	block, err := aes.NewCipher(settings.CryptoSettings.GetSecretMd5())
	if err != nil {
		return nil, fmt.Errorf("AesEncrypt invalid key: %v", err)
	}

	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("AesEncrypt unable to read IV: %w", err)
	}

	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))

	return ciphertext, nil
}

// AesDecrypt decrypts text and given key with AES.
func AesDecrypt(text []byte) ([]byte, error) {
	block, err := aes.NewCipher(settings.CryptoSettings.GetSecretMd5())
	if err != nil {
		return nil, err
	}

	if len(text) < aes.BlockSize {
		return nil, errors.New("AesDecrypt ciphertext too short")
	}

	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)

	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, fmt.Errorf("AesDecrypt invalid decrypted base64 string: %w", err)
	}

	return data, nil
}
