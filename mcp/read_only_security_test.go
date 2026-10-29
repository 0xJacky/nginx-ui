package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
)

const mcpTestPrivateKey = "-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg\n-----END PRIVATE KEY-----\n"

type mcpReadOnlyFixture struct {
	router     http.Handler
	keyPath    string
	userID     uint64
	adminToken string
}

func setupMCPReadOnlyFixture(t *testing.T) mcpReadOnlyFixture {
	t.Helper()
	router, adminToken, userID := setupMCPSecurityRouter(t)
	internalmcp.Init(context.Background())
	require.NoError(t, model.UseDB().AutoMigrate(&model.Config{}, &model.ConfigBackup{}))

	previousNginx := *settings.NginxSettings
	t.Cleanup(func() { *settings.NginxSettings = previousNginx })
	root := t.TempDir()
	settings.NginxSettings.ConfigDir = root
	settings.NginxSettings.ConfigPath = filepath.Join(root, "nginx.conf")
	settings.NginxSettings.HostMode = ""
	settings.NginxSettings.ContainerName = ""
	confDir := nginx.GetConfPath()
	require.NoError(t, os.MkdirAll(filepath.Join(confDir, "ssl"), 0o700))
	keyPath := filepath.Join(confDir, "ssl", "example.key")
	require.NoError(t, os.WriteFile(keyPath, []byte(mcpTestPrivateKey), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(confDir, "nginx.conf"), []byte("events {}\n"), 0o600))
	require.NoError(t, model.UseDB().Create(&model.ConfigBackup{
		Name:     "example.key",
		FilePath: keyPath,
		Content:  mcpTestPrivateKey,
	}).Error)

	return mcpReadOnlyFixture{router: router, keyPath: keyPath, userID: userID, adminToken: adminToken}
}

// callMCPTool opens a Streamable HTTP session with token and calls one tool,
// returning the raw JSON-RPC response body.
func callMCPTool(t *testing.T, router http.Handler, token, tool string, arguments map[string]string) string {
	t.Helper()
	request := func(body, sessionID string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
			req.Header.Set("MCP-Protocol-Version", "2025-03-26")
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}

	initialized := request(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"read-only-test","version":"1.0.0"}}}`, "")
	require.Equal(t, http.StatusOK, initialized.Code, initialized.Body.String())
	sessionID := initialized.Header().Get("Mcp-Session-Id")
	require.NotEmpty(t, sessionID)
	request(`{"jsonrpc":"2.0","method":"notifications/initialized"}`, sessionID)

	params, err := json.Marshal(map[string]any{"name": tool, "arguments": arguments})
	require.NoError(t, err)
	called := request(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":`+string(params)+`}`, sessionID)
	require.Equal(t, http.StatusOK, called.Code, called.Body.String())
	return called.Body.String()
}

func TestMCPReadTokenCannotReadPrivateKeyMaterial(t *testing.T) {
	fixture := setupMCPReadOnlyFixture(t)
	_, readToken, err := internalmcp.CreateServiceToken("mcp-reader", []string{model.MCPTokenScopeRead}, nil, fixture.userID)
	require.NoError(t, err)
	// API scopes do not widen MCP access, so api:write must not lift the MCP rule.
	_, mixedToken, err := internalmcp.CreateServiceToken(
		"mcp-reader-api-writer",
		[]string{model.MCPTokenScopeRead, model.APITokenScopeWrite},
		nil,
		fixture.userID,
	)
	require.NoError(t, err)

	for _, token := range []string{readToken, mixedToken} {
		body := callMCPTool(t, fixture.router, token, "nginx_config_get", map[string]string{"relative_path": "ssl/example.key"})
		require.NotContains(t, body, "PRIVATE KEY")
		require.Contains(t, body, "secret files are hidden from read-only service tokens")

		body = callMCPTool(t, fixture.router, token, "nginx_config_history", map[string]string{"filepath": fixture.keyPath})
		require.NotContains(t, body, "PRIVATE KEY")
		require.Contains(t, body, settings.RedactedSensitiveValue)

		body = callMCPTool(t, fixture.router, token, "nginx_config_get", map[string]string{"relative_path": "nginx.conf"})
		require.Contains(t, body, "events {}")
	}
}

func TestMCPWriteTokenAndAdministratorKeepPrivateKeyMaterial(t *testing.T) {
	fixture := setupMCPReadOnlyFixture(t)
	_, writeToken, err := internalmcp.CreateServiceToken("mcp-writer", []string{model.MCPTokenScopeWrite}, nil, fixture.userID)
	require.NoError(t, err)

	for _, token := range []string{writeToken, fixture.adminToken} {
		body := callMCPTool(t, fixture.router, token, "nginx_config_get", map[string]string{"relative_path": "ssl/example.key"})
		require.Contains(t, body, "PRIVATE KEY")

		body = callMCPTool(t, fixture.router, token, "nginx_config_history", map[string]string{"filepath": fixture.keyPath})
		require.Contains(t, body, "PRIVATE KEY")
		require.NotContains(t, body, settings.RedactedSensitiveValue)
	}
}
