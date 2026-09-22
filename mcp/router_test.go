package mcp

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
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
	originalCryptoSecret := settings.CryptoSettings.Secret
	originalInstanceID := settings.NodeSettings.InstanceID
	originalNodeSecret := settings.NodeSettings.Secret
	t.Cleanup(func() {
		cache.Shutdown()
		settings.AuthSettings.IPWhiteList = originalIPWhiteList
		cSettings.AppSettings.JwtSecret = originalJWTSecret
		settings.CryptoSettings.Secret = originalCryptoSecret
		settings.NodeSettings.InstanceID = originalInstanceID
		settings.NodeSettings.Secret = originalNodeSecret
		model.Use(nil)
	})

	settings.AuthSettings.IPWhiteList = nil
	cSettings.AppSettings.JwtSecret = "test-secret"
	settings.CryptoSettings.Secret = "mcp-test-crypto-root"
	settings.NodeSettings.InstanceID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	settings.NodeSettings.Secret = "legacy-mcp-secret"

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}, &model.MCPServiceToken{}))

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

func TestMCPServiceTokenScopesAndQueryRejection(t *testing.T) {
	router, userToken, userID := setupMCPSecurityRouter(t)
	_, readToken, err := internalmcp.CreateServiceToken("reader", []string{model.MCPTokenScopeRead}, nil, userID)
	require.NoError(t, err)
	_, writeToken, err := internalmcp.CreateServiceToken("writer", []string{model.MCPTokenScopeWrite}, nil, userID)
	require.NoError(t, err)
	_, apiReadToken, err := internalmcp.CreateServiceToken("api-reader", []string{model.APITokenScopeRead}, nil, userID)
	require.NoError(t, err)

	readRequest := httptest.NewRequest(http.MethodPost, "/mcp_message", bytes.NewBufferString(`{
		"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nginx_config_get"}
	}`))
	readRequest.Header.Set("Authorization", "Bearer "+readToken)
	readRecorder := httptest.NewRecorder()
	router.ServeHTTP(readRecorder, readRequest)
	assert.Equal(t, http.StatusBadRequest, readRecorder.Code)
	assert.Contains(t, readRecorder.Body.String(), "Missing sessionId")

	writeWithReadToken := httptest.NewRequest(http.MethodPost, "/mcp_message", bytes.NewBufferString(`{
		"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"nginx_config_modify"}
	}`))
	writeWithReadToken.Header.Set("Authorization", "Bearer "+readToken)
	writeWithReadRecorder := httptest.NewRecorder()
	router.ServeHTTP(writeWithReadRecorder, writeWithReadToken)
	assert.Equal(t, http.StatusForbidden, writeWithReadRecorder.Code)

	writeRequest := httptest.NewRequest(http.MethodPost, "/mcp_message", bytes.NewBufferString(`{
		"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"nginx_config_modify"}
	}`))
	writeRequest.Header.Set("Authorization", "Bearer "+writeToken)
	writeRecorder := httptest.NewRecorder()
	router.ServeHTTP(writeRecorder, writeRequest)
	assert.Equal(t, http.StatusBadRequest, writeRecorder.Code)
	assert.Contains(t, writeRecorder.Body.String(), "Missing sessionId")

	apiReadRequest := httptest.NewRequest(http.MethodPost, "/mcp_message", bytes.NewBufferString(`{
		"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"nginx_config_modify"}
	}`))
	apiReadRequest.Header.Set("Authorization", "Bearer "+apiReadToken)
	apiReadRecorder := httptest.NewRecorder()
	router.ServeHTTP(apiReadRecorder, apiReadRequest)
	assert.Equal(t, http.StatusForbidden, apiReadRecorder.Code)
	assert.JSONEq(t, `{"message":"MCP token scope is insufficient"}`, apiReadRecorder.Body.String())

	queryCredentialRequest := httptest.NewRequest(http.MethodPost, "/mcp?node_secret=leaked", nil)
	queryCredentialRequest.Header.Set("Authorization", userToken)
	queryCredentialRecorder := httptest.NewRecorder()
	router.ServeHTTP(queryCredentialRecorder, queryCredentialRequest)
	assert.Equal(t, http.StatusForbidden, queryCredentialRecorder.Code)
}

func TestMCPMessageEndpointRejectsUnsupportedOrAmbiguousBodies(t *testing.T) {
	router, userToken, userID := setupMCPSecurityRouter(t)
	_, readToken, err := internalmcp.CreateServiceToken("reader", []string{model.MCPTokenScopeRead}, nil, userID)
	require.NoError(t, err)

	tests := []struct {
		name  string
		body  string
		token string
	}{
		{
			name:  "malformed JSON",
			body:  `{"jsonrpc":"2.0","id":1,"method":"tools/call"`,
			token: readToken,
		},
		{
			name: "write call with trailing garbage and read token",
			body: `{"jsonrpc":"2.0","id":2,"method":"tools/call",` +
				`"params":{"name":"nginx_config_modify"}} trailing`,
			token: readToken,
		},
		{
			name: "write call followed by second JSON value and read token",
			body: `{"jsonrpc":"2.0","id":3,"method":"tools/call",` +
				`"params":{"name":"nginx_config_modify"}}` +
				`{"jsonrpc":"2.0","id":4,"method":"tools/list"}`,
			token: readToken,
		},
		{
			name: "batch array",
			body: `[{"jsonrpc":"2.0","id":5,"method":"tools/call",` +
				`"params":{"name":"nginx_config_get"}}]`,
			token: readToken,
		},
		{
			name: "write call with trailing garbage and no secure session",
			body: `{"jsonrpc":"2.0","id":6,"method":"tools/call",` +
				`"params":{"name":"nginx_config_modify"}} trailing`,
			token: userToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/mcp_message", bytes.NewBufferString(tt.body))
			request.Header.Set("Authorization", "Bearer "+tt.token)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.JSONEq(t, `{"message":"Invalid MCP request body"}`, recorder.Body.String())
		})
	}
}

// TestMCPLegacyHeaderRequiresTheConfiguredSecret covers the shared-secret path
// MCP keeps for nodes that have not been upgraded to signed requests yet.
func TestMCPLegacyHeaderRequiresTheConfiguredSecret(t *testing.T) {
	router, _, _ := setupMCPSecurityRouter(t)
	secret := settings.NodeSettings.Secret

	legacyRequest := func(value string) *http.Request {
		request := httptest.NewRequest(http.MethodPost, "/mcp_message", bytes.NewBufferString(`{"method":"tools/list"}`))
		request.Header.Set("X-Node-Secret", value)
		return request
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, legacyRequest(secret))
	assert.NotEqual(t, http.StatusForbidden, recorder.Code)

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, legacyRequest(secret+"-wrong"))
	assert.Equal(t, http.StatusForbidden, recorder.Code)

	// An instance without a configured secret must never treat an empty or
	// arbitrary header as a credential.
	settings.NodeSettings.Secret = ""
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, legacyRequest(secret))
	assert.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestMCPExpiredAndRevokedServiceTokensAreRejected(t *testing.T) {
	router, _, userID := setupMCPSecurityRouter(t)
	expiresAt := time.Now().Add(time.Minute)
	record, token, err := internalmcp.CreateServiceToken("temporary", []string{model.MCPTokenScopeRead}, &expiresAt, userID)
	require.NoError(t, err)
	require.NoError(t, internalmcp.RevokeServiceToken(record.PublicID))

	request := httptest.NewRequest(http.MethodPost, "/mcp_message", bytes.NewBufferString(`{"method":"tools/list"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
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

func TestClassifyMCPRequest(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantScope string
		wantError bool
	}{
		{
			name:      "mutating config tool",
			body:      `{"method":"tools/call","params":{"name":"nginx_config_add"}}`,
			wantScope: model.MCPTokenScopeWrite,
		},
		{
			name:      "read-only config tool",
			body:      `{"method":"tools/call","params":{"name":"nginx_config_get"}}`,
			wantScope: model.MCPTokenScopeRead,
		},
		{
			name:      "unknown tool fails closed",
			body:      `{"method":"tools/call","params":{"name":"future_tool"}}`,
			wantScope: model.MCPTokenScopeWrite,
		},
		{
			name:      "non-tool request",
			body:      `{"method":"tools/list"}`,
			wantScope: model.MCPTokenScopeRead,
		},
		{
			name:      "trailing garbage",
			body:      `{"method":"tools/call","params":{"name":"nginx_config_add"}} trailing`,
			wantError: true,
		},
		{
			name:      "concatenated JSON values",
			body:      `{"method":"tools/call","params":{"name":"nginx_config_add"}}{"method":"tools/list"}`,
			wantError: true,
		},
		{
			name:      "batch array",
			body:      `[{"method":"tools/call","params":{"name":"nginx_config_add"}}]`,
			wantError: true,
		},
		{
			name:      "malformed JSON",
			body:      `{"method":"tools/call"`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope, err := classifyMCPRequest([]byte(tt.body))
			if tt.wantError {
				assert.ErrorIs(t, err, errInvalidMCPRequestBody)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantScope, scope)
		})
	}
}
