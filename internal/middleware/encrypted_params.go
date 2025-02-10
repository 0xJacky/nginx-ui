package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

var (
	e                       = cosy.NewErrorScope("middleware")
	ErrInvalidRequestFormat = e.New(40000, "invalid request format")
	ErrDecryptionFailed     = e.New(40001, "decryption failed")
)

func EncryptedParams() gin.HandlerFunc {
	return func(c *gin.Context) {
		// read the encrypted payload
		var encryptedReq struct {
			EncryptedParams string `json:"encrypted_params"`
		}

		if err := c.ShouldBindJSON(&encryptedReq); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, ErrInvalidRequestFormat)
			return
		}

		// decrypt the parameters
		decryptedData, err := crypto.Decrypt(encryptedReq.EncryptedParams)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, ErrDecryptionFailed)
			return
		}

		// replace request body with decrypted data
		newBody, _ := json.Marshal(decryptedData)
		c.Request.Body = io.NopCloser(bytes.NewReader(newBody))
		c.Request.ContentLength = int64(len(newBody))

		c.Next()
	}
}
