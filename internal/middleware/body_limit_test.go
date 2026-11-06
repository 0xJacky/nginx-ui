package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	cSettings "github.com/uozi-tech/cosy/settings"
)

func TestScopedBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := cSettings.ServerSettings.PayloadMaxBytes
	cSettings.ServerSettings.PayloadMaxBytes = 0
	t.Cleanup(func() { cSettings.ServerSettings.PayloadMaxBytes = previous })

	r := gin.New()
	// The cosy cap, applied to every route before anything else runs.
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cSettings.ServerSettings.PayloadLimit())
		c.Next()
	})
	r.Use(ScopedBodyLimit())
	read := func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusRequestEntityTooLarge, err.Error())
			return
		}
		c.Status(http.StatusOK)
	}
	r.POST("/api/plugins", read)
	r.POST("/api/other", read)
	handler := LargeUploads(r)

	big := bytes.Repeat([]byte("x"), int(cSettings.DefaultPayloadMaxBytes)+1)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/other", bytes.NewReader(big)))
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/plugins", bytes.NewReader(big)))
	assert.Equal(t, http.StatusOK, w.Code)

	// Only a POST is an upload.
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/plugins", bytes.NewReader(big)))
	assert.NotEqual(t, http.StatusOK, w.Code)

	// The configured cap is never touched, so a settings save writes it
	// back unchanged.
	assert.Equal(t, int64(0), cSettings.ServerSettings.PayloadMaxBytes)
}
