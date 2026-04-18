package settings

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	cSettings "github.com/uozi-tech/cosy/settings"
)

func TestSaveSettingsRejectsNegativeLogrotateInterval(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/settings",
		bytes.NewBufferString(`{
			"auth":{"ban_threshold_minutes":1,"max_attempts":1},
			"cert":{"renewal_interval":7},
			"logrotate":{"enabled":true,"interval":-1}
		}`))
	c.Request.Header.Set("Content-Type", "application/json")

	SaveSettings(c)

	assert.Equal(t, http.StatusNotAcceptable, w.Code)
	assert.Contains(t, w.Body.String(), "\"interval\":\"min\"")
}

func TestGetSettingsRedactsSensitiveFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalJWTSecret := cSettings.AppSettings.JwtSecret
	originalPageSize := cSettings.AppSettings.PageSize
	originalNodeSecret := appsettings.NodeSettings.Secret
	originalNodeName := appsettings.NodeSettings.Name
	originalOpenAIToken := appsettings.OpenAISettings.Token
	originalReloadCmd := appsettings.NginxSettings.ReloadCmd
	originalRestartCmd := appsettings.NginxSettings.RestartCmd
	defer func() {
		cSettings.AppSettings.JwtSecret = originalJWTSecret
		cSettings.AppSettings.PageSize = originalPageSize
		appsettings.NodeSettings.Secret = originalNodeSecret
		appsettings.NodeSettings.Name = originalNodeName
		appsettings.OpenAISettings.Token = originalOpenAIToken
		appsettings.NginxSettings.ReloadCmd = originalReloadCmd
		appsettings.NginxSettings.RestartCmd = originalRestartCmd
	}()

	cSettings.AppSettings.JwtSecret = "jwt-secret"
	cSettings.AppSettings.PageSize = 50
	appsettings.NodeSettings.Secret = "node-secret"
	appsettings.NodeSettings.Name = "local-node"
	appsettings.OpenAISettings.Token = "openai-secret"
	appsettings.NginxSettings.ReloadCmd = "nginx -s reload"
	appsettings.NginxSettings.RestartCmd = "nginx -s restart"

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/settings", nil)

	GetSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, redactedSensitiveValue, body["app"]["jwt_secret"])
	assert.Equal(t, float64(50), body["app"]["page_size"])
	assert.Equal(t, redactedSensitiveValue, body["node"]["secret"])
	assert.Equal(t, "local-node", body["node"]["name"])
	assert.Equal(t, redactedSensitiveValue, body["openai"]["token"])
}

func TestRestoreRedactedSensitiveSettings(t *testing.T) {
	originalJWTSecret := cSettings.AppSettings.JwtSecret
	originalNodeSecret := appsettings.NodeSettings.Secret
	originalOpenAIToken := appsettings.OpenAISettings.Token
	defer func() {
		cSettings.AppSettings.JwtSecret = originalJWTSecret
		appsettings.NodeSettings.Secret = originalNodeSecret
		appsettings.OpenAISettings.Token = originalOpenAIToken
	}()

	cSettings.AppSettings.JwtSecret = "jwt-secret"
	appsettings.NodeSettings.Secret = "node-secret"
	appsettings.OpenAISettings.Token = "openai-secret"

	payload := saveSettingsPayload{}
	payload.App.JwtSecret = redactedSensitiveValue
	payload.Node.Secret = redactedSensitiveValue
	payload.Openai.Token = redactedSensitiveValue

	restoreRedactedSensitiveSettings(&payload)

	assert.Equal(t, "jwt-secret", payload.App.JwtSecret)
	assert.Equal(t, "node-secret", payload.Node.Secret)
	assert.Equal(t, "openai-secret", payload.Openai.Token)
}

func TestGetProtectedSetting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()
	defer cache.Shutdown()

	originalJWTSecret := cSettings.AppSettings.JwtSecret
	defer func() {
		cSettings.AppSettings.JwtSecret = originalJWTSecret
	}()
	cSettings.AppSettings.JwtSecret = "jwt-secret"

	t.Run("rejects missing secure session", func(t *testing.T) {
		r := gin.New()
		r.GET("/api/settings/protected", func(c *gin.Context) {
			c.Set("user", &model.User{
				Model:     model.Model{ID: 1},
				OTPSecret: []byte("otp-enabled"),
			})
		}, middleware.RequireSecureSession(), GetProtectedSetting)

		req := httptest.NewRequest(http.MethodGet, "/api/settings/protected?path=app.jwt_secret", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("rejects node secret authentication", func(t *testing.T) {
		r := gin.New()
		r.GET("/api/settings/protected", func(c *gin.Context) {
			c.Set("user", &model.User{
				Model: model.Model{ID: 1},
			})
			c.Set("Secret", "node-secret")
		}, middleware.RequireSecureSession(), GetProtectedSetting)

		req := httptest.NewRequest(http.MethodGet, "/api/settings/protected?path=app.jwt_secret", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		r := gin.New()
		r.GET("/api/settings/protected", func(c *gin.Context) {
			user := &model.User{
				Model:     model.Model{ID: 2},
				OTPSecret: []byte("otp-enabled"),
			}
			c.Set("user", user)
		}, middleware.RequireSecureSession(), GetProtectedSetting)

		req := httptest.NewRequest(http.MethodGet, "/api/settings/protected?path=node.name", nil)
		req.Header.Set("X-Secure-Session-ID", internaluser.SetSecureSessionID(2))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns protected value", func(t *testing.T) {
		r := gin.New()
		r.GET("/api/settings/protected", func(c *gin.Context) {
			user := &model.User{
				Model:     model.Model{ID: 3},
				OTPSecret: []byte("otp-enabled"),
			}
			c.Set("user", user)
		}, middleware.RequireSecureSession(), GetProtectedSetting)

		req := httptest.NewRequest(http.MethodGet, "/api/settings/protected?path=app.jwt_secret", nil)
		req.Header.Set("X-Secure-Session-ID", internaluser.SetSecureSessionID(3))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var body map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &body)
		assert.NoError(t, err)
		assert.Equal(t, "jwt-secret", body["value"])
	})
}
