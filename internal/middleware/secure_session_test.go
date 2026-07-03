package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

func TestEnsureSecureSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/login", func(c *gin.Context) {
		EnsureSecureSessionCookie(c)
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
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
}

func TestRequireSecureSessionAppliesToPasskeyOnlyUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Passkey{}))
	model.Use(db)

	passkeyUser := &model.User{Model: model.Model{ID: 1}, Name: "passkey", Status: true}
	require.NoError(t, db.Create(passkeyUser).Error)
	require.NoError(t, db.Create(&model.Passkey{UserID: passkeyUser.ID, Name: "key"}).Error)

	router := gin.New()
	router.POST("/sensitive", func(c *gin.Context) {
		c.Set("user", passkeyUser)
		c.Next()
	}, RequireSecureSession(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/sensitive", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	sessionID := internaluser.SetSecureSessionID(passkeyUser.ID)
	req = httptest.NewRequest(http.MethodPost, "/sensitive", nil)
	req.Header.Set("X-Secure-Session-ID", sessionID)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}
