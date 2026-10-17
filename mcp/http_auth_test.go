package mcp

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func requestMCP(t *testing.T, router *gin.Engine, endpoint, authorization, tool, secureSession string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, endpoint,
		bytes.NewBufferString(`{"method":"tools/call","params":{"name":"`+tool+`"}}`))
	request.Header.Set("Authorization", authorization)
	if secureSession != "" {
		request.Header.Set("X-Secure-Session-ID", secureSession)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestMCPHTTPRejectsWebSocketShortTokens(t *testing.T) {
	router, token, userID := setupMCPSecurityRouter(t)
	shortToken, err := internaluser.GenerateShortTokenForSession(userID, token)
	require.NoError(t, err)
	_, valid := internaluser.GetTokenUserByShortToken(shortToken)
	require.True(t, valid, "the fixture must be a valid WebSocket session token")
	secureSession := internaluser.SetSecureSessionID(userID)

	for _, endpoint := range []string{"/mcp", "/mcp_message"} {
		for _, authorization := range []string{shortToken, "Bearer " + shortToken} {
			for _, tool := range []string{"nginx_config_get", "nginx_config_modify"} {
				t.Run(endpoint+"/"+authorization[:6]+"/"+tool, func(t *testing.T) {
					recorder := requestMCP(t, router, endpoint, authorization, tool, secureSession)
					require.Equal(t, http.StatusForbidden, recorder.Code)
					require.JSONEq(t, `{"message":"Authorization failed"}`, recorder.Body.String())
				})
			}
		}
	}

	_, valid = internaluser.GetTokenUserByShortToken(shortToken)
	require.True(t, valid, "HTTP rejection must not revoke the WebSocket credential")
}

func TestMCPHTTPKeepsSupportedCredentialsAndWriteChecks(t *testing.T) {
	router, token, userID := setupMCPSecurityRouter(t)
	_, readToken, err := internalmcp.CreateServiceToken("http-reader", []string{model.MCPTokenScopeRead}, nil, userID)
	require.NoError(t, err)
	_, writeToken, err := internalmcp.CreateServiceToken("http-writer", []string{model.MCPTokenScopeWrite}, nil, userID)
	require.NoError(t, err)
	secureSession := internaluser.SetSecureSessionID(userID)

	for _, endpoint := range []string{"/mcp", "/mcp_message"} {
		t.Run(endpoint, func(t *testing.T) {
			// Missing content type (Streamable HTTP) or SSE session makes accepted
			// credentials reach transport validation without executing a real tool.
			transportMessage := "Invalid content type"
			if endpoint == "/mcp_message" {
				transportMessage = "Missing sessionId"
			}
			assertTransportResponse := func(recorder *httptest.ResponseRecorder) {
				t.Helper()
				require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
				require.Contains(t, recorder.Body.String(), transportMessage)
			}
			for _, authorization := range []string{token, "Bearer " + token, "Bearer " + readToken, "Bearer " + writeToken} {
				recorder := requestMCP(t, router, endpoint, authorization, "nginx_config_get", "")
				assertTransportResponse(recorder)
			}

			recorder := requestMCP(t, router, endpoint, token, "nginx_config_modify", "")
			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			recorder = requestMCP(t, router, endpoint, token, "nginx_config_modify", secureSession)
			assertTransportResponse(recorder)
			recorder = requestMCP(t, router, endpoint, readToken, "nginx_config_modify", "")
			require.Equal(t, http.StatusForbidden, recorder.Code)
			recorder = requestMCP(t, router, endpoint, writeToken, "nginx_config_modify", "")
			assertTransportResponse(recorder)
			recorder = requestMCP(t, router, endpoint, readToken, "future_unclassified_tool", "")
			require.Equal(t, http.StatusForbidden, recorder.Code)

			request := httptest.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(`{"method":"tools/list"}`))
			request.Header.Set("X-Node-Secret", settings.NodeSettings.Secret)
			recorder = httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			assertTransportResponse(recorder)

			for _, badToken := range []string{"", "invalid", "invalid-long-jwt-credential", "nui_pat_invalid"} {
				recorder = requestMCP(t, router, endpoint, badToken, "nginx_config_get", "")
				require.Equal(t, http.StatusForbidden, recorder.Code)
				require.JSONEq(t, `{"message":"Authorization failed"}`, recorder.Body.String())
			}
		})
	}
}

func TestMCPHTTPRejectsRevokedInteractiveSession(t *testing.T) {
	router, token, _ := setupMCPSecurityRouter(t)
	recorder := requestMCP(t, router, "/mcp_message", token, "nginx_config_get", "")
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Missing sessionId")
	require.NoError(t, internaluser.RevokeSessionToken(token))
	for _, endpoint := range []string{"/mcp", "/mcp_message"} {
		recorder = requestMCP(t, router, endpoint, token, "nginx_config_get", "")
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.JSONEq(t, `{"message":"Authorization failed"}`, recorder.Body.String())
	}
}
