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

	originalApp := *cSettings.AppSettings
	originalAuth := *appsettings.AuthSettings
	originalCasdoor := *appsettings.CasdoorSettings
	originalCert := *appsettings.CertSettings
	originalHTTP := *appsettings.HTTPSettings
	originalLogrotate := *appsettings.LogrotateSettings
	originalNginx := *appsettings.NginxSettings
	originalNode := *appsettings.NodeSettings
	originalOIDC := *appsettings.OIDCSettings
	originalOpenAI := *appsettings.OpenAISettings
	originalTerminal := *appsettings.TerminalSettings
	defer func() {
		*cSettings.AppSettings = originalApp
		*appsettings.AuthSettings = originalAuth
		*appsettings.CasdoorSettings = originalCasdoor
		*appsettings.CertSettings = originalCert
		*appsettings.HTTPSettings = originalHTTP
		*appsettings.LogrotateSettings = originalLogrotate
		*appsettings.NginxSettings = originalNginx
		*appsettings.NodeSettings = originalNode
		*appsettings.OIDCSettings = originalOIDC
		*appsettings.OpenAISettings = originalOpenAI
		*appsettings.TerminalSettings = originalTerminal
	}()

	cSettings.AppSettings.JwtSecret = "jwt-secret"
	cSettings.AppSettings.PageSize = 50
	appsettings.AuthSettings.IPWhiteList = []string{"192.0.2.1"}
	appsettings.CasdoorSettings.Endpoint = "https://casdoor.example.com"
	appsettings.CasdoorSettings.ClientId = "casdoor-client-id"
	appsettings.CasdoorSettings.ClientSecret = "casdoor-secret"
	appsettings.CertSettings.Email = "admin@example.com"
	appsettings.HTTPSettings.GithubProxy = "https://proxy.example.com"
	appsettings.HTTPSettings.InsecureSkipVerify = true
	appsettings.LogrotateSettings.CMD = "logrotate /etc/logrotate.d/nginx"
	appsettings.NginxSettings.LogDirWhiteList = []string{"/var/log/nginx"}
	appsettings.NginxSettings.ReloadCmd = "nginx -s reload"
	appsettings.NginxSettings.RestartCmd = "nginx -s restart"
	appsettings.NginxSettings.TestConfigCmd = "nginx -t"
	appsettings.NodeSettings.Secret = "node-secret"
	appsettings.NodeSettings.Name = "local-node"
	appsettings.NodeSettings.SkipInstallation = true
	appsettings.OIDCSettings.ClientId = "oidc-client-id"
	appsettings.OIDCSettings.ClientSecret = "oidc-secret"
	appsettings.OIDCSettings.Endpoint = "https://oidc.example.com"
	appsettings.OpenAISettings.Token = "openai-secret"
	appsettings.OpenAISettings.Model = "gpt-test"
	appsettings.TerminalSettings.StartCmd = "login"

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
	assert.Equal(t, true, body["node"]["skip_installation"])
	assert.Equal(t, redactedSensitiveValue, body["openai"]["token"])
	assert.Equal(t, "gpt-test", body["openai"]["model"])
	assert.Equal(t, redactedSensitiveValue, body["casdoor"]["endpoint"])
	assert.Equal(t, redactedSensitiveValue, body["casdoor"]["client_id"])
	assert.Equal(t, redactedSensitiveValue, body["casdoor"]["client_secret"])
	assert.Equal(t, redactedSensitiveValue, body["oidc"]["client_id"])
	assert.Equal(t, redactedSensitiveValue, body["oidc"]["client_secret"])
	assert.Equal(t, redactedSensitiveValue, body["oidc"]["endpoint"])
	assert.Equal(t, redactedSensitiveValue, body["cert"]["email"])
	assert.Equal(t, "https://proxy.example.com", body["http"]["github_proxy"])
	assert.Equal(t, true, body["http"]["insecure_skip_verify"])
	assert.Equal(t, redactedSensitiveValue, body["logrotate"]["cmd"])
	assert.Equal(t, redactedSensitiveValue, body["nginx"]["reload_cmd"])
	assert.Equal(t, redactedSensitiveValue, body["nginx"]["restart_cmd"])
	assert.Equal(t, redactedSensitiveValue, body["nginx"]["test_config_cmd"])
	assert.Equal(t, []any{redactedSensitiveValue}, body["nginx"]["log_dir_white_list"])
	assert.Equal(t, []any{redactedSensitiveValue}, body["auth"]["ip_white_list"])
	assert.Equal(t, redactedSensitiveValue, body["terminal"]["start_cmd"])
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
	originalCasdoor := *appsettings.CasdoorSettings
	originalNodeSecret := appsettings.NodeSettings.Secret
	originalOIDC := *appsettings.OIDCSettings
	originalOpenAIToken := appsettings.OpenAISettings.Token
	defer func() {
		cSettings.AppSettings.JwtSecret = originalJWTSecret
		*appsettings.CasdoorSettings = originalCasdoor
		appsettings.NodeSettings.Secret = originalNodeSecret
		*appsettings.OIDCSettings = originalOIDC
		appsettings.OpenAISettings.Token = originalOpenAIToken
	}()
	cSettings.AppSettings.JwtSecret = "jwt-secret"
	appsettings.CasdoorSettings.ClientSecret = "casdoor-secret"
	appsettings.NodeSettings.Secret = "node-secret"
	appsettings.OIDCSettings.ClientSecret = "oidc-secret"
	appsettings.OpenAISettings.Token = "openai-secret"

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

	t.Run("rejects users without 2fa", func(t *testing.T) {
		r := gin.New()
		r.GET("/api/settings/protected", func(c *gin.Context) {
			c.Set("user", &model.User{
				Model: model.Model{ID: 5},
			})
		}, middleware.RequireSecureSession(), GetProtectedSetting)

		req := httptest.NewRequest(http.MethodGet, "/api/settings/protected?path=app.jwt_secret", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
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

	t.Run("returns reflected protected values", func(t *testing.T) {
		testCases := map[string]string{
			"casdoor.client_secret": "casdoor-secret",
			"node.secret":           "node-secret",
			"oidc.client_secret":    "oidc-secret",
			"openai.token":          "openai-secret",
		}

		for path, want := range testCases {
			t.Run(path, func(t *testing.T) {
				r := gin.New()
				r.GET("/api/settings/protected", func(c *gin.Context) {
					user := &model.User{
						Model:     model.Model{ID: 6},
						OTPSecret: []byte("otp-enabled"),
					}
					c.Set("user", user)
				}, middleware.RequireSecureSession(), GetProtectedSetting)

				req := httptest.NewRequest(http.MethodGet, "/api/settings/protected?path="+path, nil)
				req.Header.Set("X-Secure-Session-ID", internaluser.SetSecureSessionID(6))
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)

				var body map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &body)
				assert.NoError(t, err)
				assert.Equal(t, want, body["value"])
			})
		}
	})
}

func TestRemoveBannedIPRequiresSecureSessionForOTPUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	group := r.Group("/api", func(c *gin.Context) {
		c.Set("user", &model.User{
			Model:     model.Model{ID: 4},
			OTPSecret: []byte("otp-enabled"),
		})
		c.Next()
	})
	InitRouter(group)

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/auth/banned_ip", bytes.NewBufferString(`{"ip":"192.0.2.1"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
