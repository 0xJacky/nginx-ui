package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"reflect"

	"github.com/0xJacky/Nginx-UI/settings"
	"gorm.io/gorm/schema"
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

type JSONAesSerializer struct{}

func (JSONAesSerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue interface{}) (err error) {
	fieldValue := reflect.New(field.FieldType)

	if dbValue != nil {
		var bytes []byte
		switch v := dbValue.(type) {
		case []byte:
			bytes = v
		case string:
			bytes = []byte(v)
		default:
			bytes, err = json.Marshal(v)
			if err != nil {
				return err
			}
		}

		if len(bytes) > 0 {
			bytes, err = AesDecrypt(bytes)
			if err != nil {
				return err
			}
			err = json.Unmarshal(bytes, fieldValue.Interface())
		}
	}

	field.ReflectValueOf(ctx, dst).Set(fieldValue.Elem())
	return
}

// Value implements serializer interface
func (JSONAesSerializer) Value(ctx context.Context, field *schema.Field, dst reflect.Value, fieldValue interface{}) (interface{}, error) {
	result, err := json.Marshal(fieldValue)
	if string(result) == "null" {
		if field.TagSettings["NOT NULL"] != "" {
			return "", nil
		}
		return nil, err
	}

	encrypt, err := AesEncrypt(result)
	return string(encrypt), err
}
