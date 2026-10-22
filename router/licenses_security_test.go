//go:build unembed

package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/license"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	cosyRouter "github.com/uozi-tech/cosy/router"
)

func TestLicenseInventoryRequiresHTTPAuthentication(t *testing.T) {
	originalCryptoSecret := settings.CryptoSettings.Secret
	originalInstanceID := settings.NodeSettings.InstanceID
	t.Cleanup(func() {
		settings.CryptoSettings.Secret = originalCryptoSecret
		settings.NodeSettings.InstanceID = originalInstanceID
	})
	settings.CryptoSettings.Secret = "license-inventory-test-verifier-key"
	settings.NodeSettings.InstanceID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	currentUser := setupRouterTestEnvironment(t)
	payload, err := internaluser.GenerateJWT(currentUser)
	require.NoError(t, err)
	shortToken, err := internaluser.GenerateShortTokenForSession(currentUser.ID, payload.Token)
	require.NoError(t, err)
	require.NoError(t, model.UseDB().AutoMigrate(&model.MCPServiceToken{}))
	_, reader, err := internalmcp.CreateServiceToken("license-reader", []string{model.APITokenScopeRead}, nil, currentUser.ID)
	require.NoError(t, err)
	_, writer, err := internalmcp.CreateServiceToken("license-writer", []string{model.APITokenScopeWrite}, nil, currentUser.ID)
	require.NoError(t, err)
	_, mcpReader, err := internalmcp.CreateServiceToken("mcp-only-reader", []string{model.MCPTokenScopeRead}, nil, currentUser.ID)
	require.NoError(t, err)

	info, err := license.GetLicenseInfo()
	require.NoError(t, err)
	stats, err := license.GetLicenseStats()
	require.NoError(t, err)

	endpoints := []struct {
		path string
		want any
	}{
		{"/api/licenses", info},
		{"/api/licenses/backend", info.Backend},
		{"/api/licenses/frontend", info.Frontend},
		{"/api/licenses/stats", stats},
	}
	credentials := []struct {
		name    string
		token   string
		query   string
		cookie  string
		allowed bool
	}{
		{name: "anonymous"},
		{name: "invalid bearer", token: "not-a-valid-http-credential"},
		{name: "websocket token", token: shortToken},
		{name: "query JWT", query: "?token=" + payload.Token},
		{name: "cookie JWT", cookie: payload.Token},
		{name: "MCP-only token", token: mcpReader},
		{name: "interactive JWT", token: payload.Token, allowed: true},
		{name: "API reader", token: reader, allowed: true},
		{name: "API writer", token: writer, allowed: true},
	}
	for _, endpoint := range endpoints {
		t.Run(endpoint.path, func(t *testing.T) {
			for _, credential := range credentials {
				t.Run(credential.name, func(t *testing.T) {
					// Isolate route checks from asynchronous audit callbacks retaining
					// a pooled Gin context after an earlier request completes.
					cosyRouter.Init()
					InitRouter()
					request := httptest.NewRequest(http.MethodGet, endpoint.path+credential.query, nil)
					// Inventory belongs to this instance even when the UI selects a peer.
					request.Header.Set("X-Node-ID", "999999")
					if credential.token != "" {
						request.Header.Set("Authorization", "Bearer "+credential.token)
					}
					if credential.cookie != "" {
						request.AddCookie(&http.Cookie{Name: "token", Value: credential.cookie})
					}
					response := httptest.NewRecorder()
					cosyRouter.GetEngine().ServeHTTP(response, request)
					if !credential.allowed {
						require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
						require.JSONEq(t, `{"message":"Authorization failed"}`, response.Body.String())
						return
					}
					require.Equal(t, http.StatusOK, response.Code, response.Body.String())
					want, err := json.Marshal(endpoint.want)
					require.NoError(t, err)
					require.JSONEq(t, string(want), response.Body.String())
				})
			}
		})
	}
}
