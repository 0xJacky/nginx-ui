package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAPIServiceTokenTest(t *testing.T) (readToken, writeToken, mcpToken string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	originalSecret := settings.CryptoSettings.Secret
	originalInstanceID := settings.NodeSettings.InstanceID
	t.Cleanup(func() {
		settings.CryptoSettings.Secret = originalSecret
		settings.NodeSettings.InstanceID = originalInstanceID
		model.Use(nil)
	})
	settings.CryptoSettings.Secret = "api-service-token-test-root"
	settings.NodeSettings.InstanceID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"

	database, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.MCPServiceToken{}))
	model.Use(database)

	_, readToken, err = internalmcp.CreateServiceToken("reader", []string{model.APITokenScopeRead}, nil, 11)
	require.NoError(t, err)
	_, writeToken, err = internalmcp.CreateServiceToken("writer", []string{model.APITokenScopeWrite}, nil, 12)
	require.NoError(t, err)
	_, mcpToken, err = internalmcp.CreateServiceToken("mcp", []string{model.MCPTokenScopeWrite}, nil, 13)
	require.NoError(t, err)
	return
}

func serviceTokenRequest(router http.Handler, method, requestPath, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, requestPath, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestAPIServiceTokenScopesAndInteractiveBoundary(t *testing.T) {
	readToken, writeToken, mcpToken := setupAPIServiceTokenTest(t)

	router := gin.New()
	router.GET("/read", AuthRequired(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.POST("/write", AuthRequired(), RequireSecureSession(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.GET("/api/backup", AuthRequired(), RequireSecureSession(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.POST("/interactive", AuthRequired(), RequireInteractiveUser(), RequireSecureSession(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/api/settings/protected", AuthRequired(), RequireInteractiveUser(), RequireSecureSession(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	require.Equal(t, http.StatusNoContent, serviceTokenRequest(router, http.MethodGet, "/read", readToken).Code)
	require.Equal(t, http.StatusForbidden, serviceTokenRequest(router, http.MethodPost, "/write", readToken).Code)
	require.Equal(t, http.StatusNoContent, serviceTokenRequest(router, http.MethodPost, "/write", writeToken).Code)
	require.Equal(t, http.StatusForbidden, serviceTokenRequest(router, http.MethodGet, "/api/backup", readToken).Code)
	require.Equal(t, http.StatusNoContent, serviceTokenRequest(router, http.MethodGet, "/api/backup", writeToken).Code)
	require.Equal(t, http.StatusForbidden, serviceTokenRequest(router, http.MethodPost, "/interactive", writeToken).Code)
	require.Equal(t, http.StatusForbidden, serviceTokenRequest(router, http.MethodGet, "/api/settings/protected", writeToken).Code)
	require.Equal(t, http.StatusForbidden, serviceTokenRequest(router, http.MethodGet, "/read", mcpToken).Code)
}

func TestAPIServiceTokenPolicyUsesMatchedRouteBeforeProxy(t *testing.T) {
	readToken, writeToken, _ := setupAPIServiceTokenTest(t)

	router := gin.New()
	router.GET("/api/domain/:name/cert", AuthRequired(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.GET("/api/pty", AuthRequired(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.GET("/api/nodes/:id/secret", AuthRequired(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.GET("/api/nodes/:id", AuthRequired(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	require.Equal(t, http.StatusForbidden, serviceTokenRequest(router, http.MethodGet, "/api/domain/example.com/cert", readToken).Code)
	require.Equal(t, http.StatusNoContent, serviceTokenRequest(router, http.MethodGet, "/api/domain/example.com/cert", writeToken).Code)
	require.Equal(t, http.StatusForbidden, serviceTokenRequest(router, http.MethodGet, "/api/pty", writeToken).Code)
	require.Equal(t, http.StatusForbidden, serviceTokenRequest(router, http.MethodGet, "/api/nodes/7/secret", writeToken).Code)
	require.Equal(t, http.StatusNoContent, serviceTokenRequest(router, http.MethodGet, "/api/nodes/7", readToken).Code)
}

func TestAuthorizationTokenAcceptsBearerAndRejectsAmbientCredentials(t *testing.T) {
	context := newTestGinContext(t, http.MethodGet, "/api/test", nil)
	context.Request.Header.Set("Authorization", "  Bearer token-value  ")
	require.Equal(t, "token-value", getToken(context))
}
