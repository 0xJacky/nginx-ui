package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSecureSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("sets cookie when not present", func(t *testing.T) {
		r := gin.New()
		r.Use(SecureSessionCookie())
		r.GET("/", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		cookies := w.Result().Cookies()
		var found *http.Cookie
		for _, c := range cookies {
			if c.Name == SecureSessionCookieName {
				found = c
				break
			}
		}

		assert.NotNil(t, found, "session cookie should be set")
		assert.NotEmpty(t, found.Value)
		assert.True(t, found.HttpOnly, "cookie must be HttpOnly")
		assert.Equal(t, http.SameSiteLaxMode, found.SameSite, "cookie must be SameSite=Lax")
		assert.Equal(t, "/", found.Path)
	})

	t.Run("does not overwrite existing cookie", func(t *testing.T) {
		r := gin.New()
		r.Use(SecureSessionCookie())
		r.GET("/", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: SecureSessionCookieName, Value: "existing-value"})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		cookies := w.Result().Cookies()
		for _, c := range cookies {
			if c.Name == SecureSessionCookieName {
				t.Fatal("should not set a new cookie when one already exists")
			}
		}
	})

	t.Run("cookie value is 32 hex chars", func(t *testing.T) {
		r := gin.New()
		r.Use(SecureSessionCookie())
		r.GET("/", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		for _, c := range w.Result().Cookies() {
			if c.Name == SecureSessionCookieName {
				assert.Len(t, c.Value, 32, "hex-encoded 16 bytes = 32 chars")
				return
			}
		}
		t.Fatal("cookie not found")
	})
}
