package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serviceTokenManagementRequest(
	router http.Handler,
	method string,
	requestPath string,
	token string,
	secureSessionID string,
	body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, requestPath, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	if secureSessionID != "" {
		request.Header.Set("X-Secure-Session-ID", secureSessionID)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestServiceTokenManagementLifecycleAndInteractiveBoundary(t *testing.T) {
	_, userToken, userID := setupMCPSecurityRouter(t)
	secureSessionID := internaluser.SetSecureSessionID(userID)

	router := gin.New()
	api := router.Group("/api", middleware.AuthRequired())
	InitManagementRouter(api)

	withoutSecureSession := serviceTokenManagementRequest(
		router,
		http.MethodPost,
		"/api/service_tokens",
		userToken,
		"",
		`{"name":"ci","scopes":["api:write"]}`,
	)
	require.Equal(t, http.StatusUnauthorized, withoutSecureSession.Code)

	created := serviceTokenManagementRequest(
		router,
		http.MethodPost,
		"/api/service_tokens",
		userToken,
		secureSessionID,
		`{"name":"ci","scopes":["api:write","mcp:read"]}`,
	)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var createResponse serviceTokenResponse
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &createResponse))
	assert.Equal(t, "ci", createResponse.Name)
	assert.Equal(t, userID, createResponse.CreatorID)
	assert.True(t, strings.HasPrefix(createResponse.Token, "nui_pat_"))

	list := serviceTokenManagementRequest(router, http.MethodGet, "/api/service_tokens", userToken, "", "")
	require.Equal(t, http.StatusOK, list.Code)
	assert.NotContains(t, list.Body.String(), createResponse.Token)

	legacyList := serviceTokenManagementRequest(router, http.MethodGet, "/api/mcp/tokens", userToken, "", "")
	require.Equal(t, http.StatusOK, legacyList.Code)

	serviceTokenList := serviceTokenManagementRequest(
		router,
		http.MethodGet,
		"/api/service_tokens",
		createResponse.Token,
		"",
		"",
	)
	require.Equal(t, http.StatusForbidden, serviceTokenList.Code)

	rotated := serviceTokenManagementRequest(
		router,
		http.MethodPost,
		"/api/service_tokens/"+createResponse.ID+"/rotate",
		userToken,
		secureSessionID,
		"",
	)
	require.Equal(t, http.StatusOK, rotated.Code, rotated.Body.String())
	var rotateResponse serviceTokenResponse
	require.NoError(t, json.Unmarshal(rotated.Body.Bytes(), &rotateResponse))
	assert.NotEqual(t, createResponse.Token, rotateResponse.Token)
	_, err := internalmcp.VerifyServiceToken(createResponse.Token, time.Now())
	require.Error(t, err)

	revoked := serviceTokenManagementRequest(
		router,
		http.MethodDelete,
		"/api/service_tokens/"+createResponse.ID,
		userToken,
		secureSessionID,
		"",
	)
	require.Equal(t, http.StatusNoContent, revoked.Code)
	_, err = internalmcp.VerifyServiceToken(rotateResponse.Token, time.Now())
	require.Error(t, err)
}

func TestCreateServiceTokenRejectsUnsupportedScope(t *testing.T) {
	_, userToken, userID := setupMCPSecurityRouter(t)
	router := gin.New()
	api := router.Group("/api", middleware.AuthRequired())
	InitManagementRouter(api)

	recorder := serviceTokenManagementRequest(
		router,
		http.MethodPost,
		"/api/service_tokens",
		userToken,
		internaluser.SetSecureSessionID(userID),
		`{"name":"invalid","scopes":["api:admin"]}`,
	)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "unsupported service token scope")
}

func TestServiceTokenResponseNeverSerializesVerifier(t *testing.T) {
	record := serviceTokenResponse{ID: "public-id", Name: "ci", Token: "nui_pat_secret"}
	encoded, err := json.Marshal(record)
	require.NoError(t, err)
	assert.NotContains(t, string(bytes.ToLower(encoded)), "verifier")
}
