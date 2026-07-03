package mcp

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMCPEndpointsRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalIPWhiteList := settings.AuthSettings.IPWhiteList
	t.Cleanup(func() {
		settings.AuthSettings.IPWhiteList = originalIPWhiteList
	})

	settings.AuthSettings.IPWhiteList = nil

	router := gin.New()
	InitRouter(router)

	for _, endpoint := range []string{"/mcp", "/mcp_message"} {
		req := httptest.NewRequest(http.MethodPost, endpoint, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.JSONEq(t, `{"message":"Authorization failed"}`, w.Body.String())
	}
}

func setupMCPSecurityRouter(t *testing.T) (*gin.Engine, string, uint64) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()

	originalIPWhiteList := settings.AuthSettings.IPWhiteList
	originalJWTSecret := cSettings.AppSettings.JwtSecret
	t.Cleanup(func() {
		cache.Shutdown()
		settings.AuthSettings.IPWhiteList = originalIPWhiteList
		cSettings.AppSettings.JwtSecret = originalJWTSecret
	})

	settings.AuthSettings.IPWhiteList = nil
	cSettings.AppSettings.JwtSecret = "test-secret"

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}))

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	otpUser := &model.User{
		Model:     model.Model{ID: 1},
		Name:      "otp",
		Status:    true,
		Language:  "en",
		OTPSecret: []byte("otp-enabled"),
	}
	require.NoError(t, db.Create(otpUser).Error)

	payload, err := internaluser.GenerateJWT(otpUser)
	require.NoError(t, err)

	router := gin.New()
	InitRouter(router)

	return router, payload.Token, otpUser.ID
}

func TestMCPMutatingToolRequiresSecureSessionForOTPUser(t *testing.T) {
	router, token, _ := setupMCPSecurityRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/mcp_message", bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {"name": "nginx_config_modify"}
	}`))
	req.Header.Set("Authorization", token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.JSONEq(t, `{"message":"Secure Session ID is empty"}`, w.Body.String())
}

func TestMCPMutatingToolAllowsValidSecureSessionForOTPUser(t *testing.T) {
	router, token, userID := setupMCPSecurityRouter(t)
	sessionID := internaluser.SetSecureSessionID(userID)

	req := httptest.NewRequest(http.MethodPost, "/mcp_message", bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {"name": "nginx_config_modify"}
	}`))
	req.Header.Set("Authorization", token)
	req.Header.Set("X-Secure-Session-ID", sessionID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
}

func TestMCPReadOnlyToolDoesNotRequireSecureSessionForOTPUser(t *testing.T) {
	router, token, _ := setupMCPSecurityRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/mcp_message", bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {"name": "nginx_config_get"}
	}`))
	req.Header.Set("Authorization", token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
}

func TestMCPRequestNeedsSecureSession(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "mutating config tool",
			body: `{"method":"tools/call","params":{"name":"nginx_config_add"}}`,
			want: true,
		},
		{
			name: "read-only config tool",
			body: `{"method":"tools/call","params":{"name":"nginx_config_get"}}`,
			want: false,
		},
		{
			name: "batch containing mutating tool",
			body: `[
				{"method":"tools/call","params":{"name":"nginx_config_list"}},
				{"method":"tools/call","params":{"name":"restart_nginx"}}
			]`,
			want: true,
		},
		{
			name: "non-tool request",
			body: `{"method":"tools/list"}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mcpRequestNeedsSecureSession([]byte(tt.body)))
		})
	}
}
