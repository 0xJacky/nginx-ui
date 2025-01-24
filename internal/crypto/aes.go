package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"github.com/0xJacky/Nginx-UI/settings"
	"io"
)

// AesEncrypt encrypts text and given key with AES.
func AesEncrypt(text []byte) ([]byte, error) {
	if len(text) == 0 {
		return nil, ErrPlainTextEmpty
	}
	block, err := aes.NewCipher(settings.CryptoSettings.GetSecretMd5())
	if err != nil {
		return nil, err
	}

	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
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
		return nil, ErrCipherTextTooShort
	}

	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)

	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, err
	}

	return data, nil
}
