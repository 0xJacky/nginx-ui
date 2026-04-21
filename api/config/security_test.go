package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalconfig "github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type configAuthFixture struct {
	plainToken string
	otpToken   string
}

type cosyErrorResponse struct {
	Scope   string   `json:"scope"`
	Code    int32    `json:"code"`
	Message string   `json:"message"`
	Params  []string `json:"params"`
}

func mustCosyErrorMeta(t *testing.T, err error) (scope string, code int32) {
	t.Helper()

	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("expected cosy error, got %v", err)
	}

	return cosyErr.Scope, cosyErr.Code
}

func assertCosyErrorResponse(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
	wantScope string,
	wantCode int32,
	wantParams ...string,
) {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf("expected %d, got %d", wantStatus, recorder.Code)
	}

	var response cosyErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if response.Scope != wantScope {
		t.Fatalf("expected scope %q, got %q", wantScope, response.Scope)
	}

	if response.Code != wantCode {
		t.Fatalf("expected code %d, got %d", wantCode, response.Code)
	}

	if !reflect.DeepEqual(response.Params, wantParams) {
		t.Fatalf("expected params %v, got %v", wantParams, response.Params)
	}
}

func setupConfigSecurityTest(t *testing.T) (string, configAuthFixture) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()

	confDir := t.TempDir()

	originalConfigDir := appsettings.NginxSettings.ConfigDir
	originalReloadCmd := appsettings.NginxSettings.ReloadCmd
	originalRestartCmd := appsettings.NginxSettings.RestartCmd
	originalTestConfigCmd := appsettings.NginxSettings.TestConfigCmd
	originalNodeSecret := appsettings.NodeSettings.Secret
	originalJWTSecret := settings.AppSettings.JwtSecret

	appsettings.NginxSettings.ConfigDir = confDir
	appsettings.NginxSettings.ReloadCmd = "true"
	appsettings.NginxSettings.RestartCmd = "true"
	appsettings.NginxSettings.TestConfigCmd = "true"
	appsettings.NodeSettings.Secret = "node-secret"
	settings.AppSettings.JwtSecret = "test-secret"

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.AuthToken{},
		&model.Passkey{},
		&model.Config{},
		&model.ConfigBackup{},
		&model.LLMSession{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	initUser := &model.User{Model: model.Model{ID: 1}, Name: "init", Status: true, Language: "en"}
	plainUser := &model.User{Model: model.Model{ID: 2}, Name: "plain", Status: true, Language: "en"}
	otpUser := &model.User{Model: model.Model{ID: 3}, Name: "otp", Status: true, Language: "en", OTPSecret: []byte("otp-enabled")}

	for _, user := range []*model.User{initUser, plainUser, otpUser} {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("failed to create test user %s: %v", user.Name, err)
		}
	}

	plainPayload, err := internaluser.GenerateJWT(plainUser)
	if err != nil {
		t.Fatalf("failed to create plain token: %v", err)
	}

	otpPayload, err := internaluser.GenerateJWT(otpUser)
	if err != nil {
		t.Fatalf("failed to create otp token: %v", err)
	}

	t.Cleanup(func() {
		cache.Shutdown()
		appsettings.NginxSettings.ConfigDir = originalConfigDir
		appsettings.NginxSettings.ReloadCmd = originalReloadCmd
		appsettings.NginxSettings.RestartCmd = originalRestartCmd
		appsettings.NginxSettings.TestConfigCmd = originalTestConfigCmd
		appsettings.NodeSettings.Secret = originalNodeSecret
		settings.AppSettings.JwtSecret = originalJWTSecret
	})

	return confDir, configAuthFixture{
		plainToken: plainPayload.Token,
		otpToken:   otpPayload.Token,
	}
}

func newConfigMutationRouter() *gin.Engine {
	r := gin.New()
	g := r.Group("/", middleware.AuthRequired())
	InitRouter(g)
	return r
}

func performJSONRequest(t *testing.T, router http.Handler, method string, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody []byte
	var err error
	if body != nil {
		requestBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestConfigMutationsRequireSecureSessionForOTPUser(t *testing.T) {
	_, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()

	recorder := performJSONRequest(t, router, http.MethodPost, "/configs", gin.H{
		"name":    "app.conf",
		"content": "server {\n}\n",
	}, map[string]string{
		"Authorization": auth.otpToken,
	})

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestAddConfigAllowsNonOTPUserAndNodeSecret(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()

	plainRecorder := performJSONRequest(t, router, http.MethodPost, "/configs", gin.H{
		"name":    "plain.conf",
		"content": "server {\n}\n",
	}, map[string]string{
		"Authorization": auth.plainToken,
	})
	if plainRecorder.Code != http.StatusOK {
		t.Fatalf("expected plain request 200, got %d", plainRecorder.Code)
	}

	if _, err := os.Stat(filepath.Join(confDir, "plain.conf")); err != nil {
		t.Fatalf("expected plain config file: %v", err)
	}

	nodeRouter := gin.New()
	nodeRouter.POST("/configs", func(c *gin.Context) {
		c.Set("user", &model.User{Model: model.Model{ID: 1}, Name: "node-sync", Status: true})
		c.Set("Secret", "node-secret")
		c.Next()
	}, middleware.RequireSecureSession(), AddConfig)

	nodeRecorder := performJSONRequest(t, nodeRouter, http.MethodPost, "/configs", gin.H{
		"name":    "node.conf",
		"content": "server {\n}\n",
	}, nil)
	if nodeRecorder.Code != http.StatusOK {
		t.Fatalf("expected node request 200, got %d", nodeRecorder.Code)
	}

	if _, err := os.Stat(filepath.Join(confDir, "node.conf")); err != nil {
		t.Fatalf("expected node config file: %v", err)
	}
}

func TestAddConfigRejectsDisallowedFilename(t *testing.T) {
	_, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()
	scope, code := mustCosyErrorMeta(t, internalconfig.ErrConfigFilenameNotAllowed)

	recorder := performJSONRequest(t, router, http.MethodPost, "/configs", gin.H{
		"name":    "evil.so",
		"content": "server {\n}\n",
	}, map[string]string{
		"Authorization": auth.plainToken,
	})

	assertCosyErrorResponse(t, recorder, http.StatusInternalServerError, scope, code, "evil.so")
}

func TestAddConfigRejectsBinaryContent(t *testing.T) {
	_, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()
	scope, code := mustCosyErrorMeta(t, internalconfig.ErrConfigContentHasControlChars)

	recorder := performJSONRequest(t, router, http.MethodPost, "/configs", gin.H{
		"name":    "app.conf",
		"content": "server {\x00}\n",
	}, map[string]string{
		"Authorization": auth.plainToken,
	})

	assertCosyErrorResponse(t, recorder, http.StatusInternalServerError, scope, code)
}

func TestEditConfigRejectsBinaryContent(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()
	scope, code := mustCosyErrorMeta(t, internalconfig.ErrConfigContentHasControlChars)

	if err := os.WriteFile(filepath.Join(confDir, "nginx.conf"), []byte("events {}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}

	recorder := performJSONRequest(t, router, http.MethodPost, "/config", gin.H{
		"path":    "nginx.conf",
		"content": "events {\x00}\n",
	}, map[string]string{
		"Authorization": auth.plainToken,
	})

	assertCosyErrorResponse(t, recorder, http.StatusInternalServerError, scope, code)
}

func TestRenameRejectsDisallowedTargetFile(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()
	scope, code := mustCosyErrorMeta(t, internalconfig.ErrConfigFilenameNotAllowed)

	if err := os.WriteFile(filepath.Join(confDir, "nginx.conf"), []byte("events {}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}

	recorder := performJSONRequest(t, router, http.MethodPost, "/config_rename", gin.H{
		"base_path": "",
		"orig_name": "nginx.conf",
		"new_name":  "evil.so",
	}, map[string]string{
		"Authorization": auth.plainToken,
	})

	assertCosyErrorResponse(t, recorder, http.StatusInternalServerError, scope, code, "evil.so")
}

func TestRenameAllowsDirectoryRename(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()

	if err := os.MkdirAll(filepath.Join(confDir, "snippets"), 0o755); err != nil {
		t.Fatalf("failed to seed directory: %v", err)
	}

	recorder := performJSONRequest(t, router, http.MethodPost, "/config_rename", gin.H{
		"base_path": "",
		"orig_name": "snippets",
		"new_name":  "renamed-snippets",
	}, map[string]string{
		"Authorization": auth.plainToken,
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	if _, err := os.Stat(filepath.Join(confDir, "renamed-snippets")); err != nil {
		t.Fatalf("expected renamed directory: %v", err)
	}
}
