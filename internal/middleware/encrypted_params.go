package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

var (
	e                       = cosy.NewErrorScope("middleware")
	ErrInvalidRequestFormat = e.New(40000, "invalid request format")
	ErrDecryptionFailed     = e.New(40001, "decryption failed")
	ErrFormParseFailed      = e.New(40002, "form parse failed")
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
			logger.Error(err)
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

// EncryptedForm handles multipart/form-data with encrypted fields while preserving file uploads
func EncryptedForm() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only process if the content type is multipart/form-data
		if !strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
			c.Next()
			return
		}

		// Parse the multipart form
		if err := c.Request.ParseMultipartForm(512 << 20); err != nil { // 512MB max memory
			c.AbortWithStatusJSON(http.StatusBadRequest, ErrFormParseFailed)
			return
		}

		// Check if encrypted_params field exists
		encryptedParams := c.Request.FormValue("encrypted_params")
		if encryptedParams == "" {
			// No encryption, continue normally
			c.Next()
			return
		}

		// Decrypt the parameters
		params, err := crypto.Decrypt(encryptedParams)
		if err != nil {
			logger.Error(err)
			c.AbortWithStatusJSON(http.StatusBadRequest, ErrDecryptionFailed)
			return
		}

		// Create a new multipart form with the decrypted data
		newForm := &multipart.Form{
			Value: make(map[string][]string),
			File:  c.Request.MultipartForm.File, // Keep original file uploads
		}

		// Add decrypted values to the new form
		for key, val := range params {
			strVal, ok := val.(string)
			if ok {
				newForm.Value[key] = []string{strVal}
			} else {
				// Handle other types if necessary
				jsonVal, _ := json.Marshal(val)
				newForm.Value[key] = []string{string(jsonVal)}
			}
		}

		// Also copy original non-encrypted form values (except encrypted_params)
		for key, vals := range c.Request.MultipartForm.Value {
			if key != "encrypted_params" && newForm.Value[key] == nil {
				newForm.Value[key] = vals
			}
		}

		// Replace the original form with our modified one
		c.Request.MultipartForm = newForm

		// Remove the encrypted_params field from the form
		delete(c.Request.MultipartForm.Value, "encrypted_params")

		// Reset ContentLength as form structure has changed
		c.Request.ContentLength = -1

		// Sync the form values to the request PostForm to ensure Gin can access them
		if c.Request.PostForm == nil {
			c.Request.PostForm = make(url.Values)
		}

		// Copy all values from MultipartForm to PostForm
		for k, v := range newForm.Value {
			c.Request.PostForm[k] = v
		}

		c.Next()
	}
}
