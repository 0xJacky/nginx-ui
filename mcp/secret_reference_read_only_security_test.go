package mcp

import (
	"os"
	"path/filepath"
	"testing"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
)

const (
	mcpControlTicketKey = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	mcpHashTicketKey    = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	mcpIncludedPassword = "passphrase-named-by-outside-include"
)

// setupMCPSecretReferenceFixture stores files that are secret only because a
// directive names them, written the ways nginx accepts.
func setupMCPSecretReferenceFixture(t *testing.T) (fixture mcpReadOnlyFixture, hashPath string) {
	t.Helper()
	fixture = setupMCPReadOnlyFixture(t)

	confDir := nginx.GetConfPath()
	hashPath = filepath.Join(confDir, "ssl", "ticket#1.bin")
	outside := filepath.Join(t.TempDir(), "tls.conf")
	require.NoError(t, os.WriteFile(outside, []byte("ssl_password_file ssl/included.pass;\n"), 0o600))
	for path, content := range map[string]string{
		hashPath: mcpHashTicketKey,
		filepath.Join(confDir, "ssl", "ticket-control.bin"): mcpControlTicketKey,
		filepath.Join(confDir, "ssl", "included.pass"):      mcpIncludedPassword,
		filepath.Join(confDir, "ssl", "retired.txt"):        "retired",
		filepath.Join(confDir, "nginx.conf"): `events {}
http {
    ssl_session_ticket_key ssl/ticket-control.bin;
    ssl_session_ticket_key "ssl/ticket#1.bin";
    include ` + outside + `;
    # ssl_password_file ssl/retired.txt;
}
`,
	} {
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	require.NoError(t, model.UseDB().Create(&model.ConfigBackup{
		Name:     "ticket#1.bin",
		FilePath: hashPath,
		Content:  mcpHashTicketKey,
	}).Error)

	return fixture, hashPath
}

// mcp:read follows the same rule as api:read for files a secret directive
// names, however the directive is written. (GHSA-wxp5-6x82-7qf4)
func TestMCPReadTokenCannotReadSecretFileNamedByDirective(t *testing.T) {
	fixture, hashPath := setupMCPSecretReferenceFixture(t)
	_, readToken, err := internalmcp.CreateServiceToken("mcp-reader", []string{model.MCPTokenScopeRead}, nil, fixture.userID)
	require.NoError(t, err)

	for _, file := range []struct{ path, secret string }{
		{"ssl/ticket-control.bin", mcpControlTicketKey},
		{"ssl/ticket#1.bin", mcpHashTicketKey},
		{"ssl/included.pass", mcpIncludedPassword},
	} {
		t.Run(file.path, func(t *testing.T) {
			body := callMCPTool(t, fixture.router, readToken, "nginx_config_get", map[string]string{"relative_path": file.path})
			require.NotContains(t, body, file.secret)
			require.Contains(t, body, "secret files are hidden from read-only service tokens")
		})
	}

	t.Run("configuration history", func(t *testing.T) {
		body := callMCPTool(t, fixture.router, readToken, "nginx_config_history", map[string]string{"filepath": hashPath})
		require.NotContains(t, body, mcpHashTicketKey)
		require.Contains(t, body, settings.RedactedSensitiveValue)
	})

	t.Run("ordinary files stay readable", func(t *testing.T) {
		body := callMCPTool(t, fixture.router, readToken, "nginx_config_get", map[string]string{"relative_path": "nginx.conf"})
		require.Contains(t, body, "ssl_session_ticket_key")
		body = callMCPTool(t, fixture.router, readToken, "nginx_config_get", map[string]string{"relative_path": "ssl/retired.txt"})
		require.Contains(t, body, "retired")
	})
}

func TestMCPWriteTokenAndAdministratorKeepSecretFileNamedByDirective(t *testing.T) {
	fixture, hashPath := setupMCPSecretReferenceFixture(t)
	_, writeToken, err := internalmcp.CreateServiceToken("mcp-writer", []string{model.MCPTokenScopeWrite}, nil, fixture.userID)
	require.NoError(t, err)

	for _, token := range []string{writeToken, fixture.adminToken} {
		body := callMCPTool(t, fixture.router, token, "nginx_config_get", map[string]string{"relative_path": "ssl/ticket#1.bin"})
		require.Contains(t, body, mcpHashTicketKey)

		body = callMCPTool(t, fixture.router, token, "nginx_config_history", map[string]string{"filepath": hashPath})
		require.Contains(t, body, mcpHashTicketKey)
		require.NotContains(t, body, settings.RedactedSensitiveValue)
	}
}
