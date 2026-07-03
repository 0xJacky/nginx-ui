package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	internalconfig "github.com/0xJacky/Nginx-UI/internal/config"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/uozi-tech/cosy"
)

func TestNginxConfigAddRejectsRestrictedDirectiveContent(t *testing.T) {
	confDir := setupMCPConfigValidationTest(t)

	_, err := handleNginxConfigAdd(context.Background(), mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{
				"name":    "app.conf",
				"content": `server { listen 80; lua_package_path "/tmp/?.lua;;"; }`,
			},
		},
	})
	requireMCPRestrictedDirectiveError(t, err, "lua_package_path")

	path := filepath.Join(confDir, "app.conf")
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected rejected MCP add to leave %q absent, stat error: %v", path, statErr)
	}
}

func TestNginxConfigModifyRejectsStatementSeparatedRestrictedDirectiveContent(t *testing.T) {
	confDir := setupMCPConfigValidationTest(t)
	path := filepath.Join(confDir, "app.conf")
	originalContent := []byte("server { listen 80; }\n")
	if err := os.WriteFile(path, originalContent, 0644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}

	_, err := handleNginxConfigModify(context.Background(), mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{
				"relative_path": "app.conf",
				"content":       "server { listen 80; js_import app.js; }\n",
			},
		},
	})
	requireMCPRestrictedDirectiveError(t, err, "js_import")

	content, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("failed to read config file: %v", readErr)
	}
	if string(content) != string(originalContent) {
		t.Fatalf("expected rejected MCP modify to keep original content %q, got %q", originalContent, content)
	}
}

func setupMCPConfigValidationTest(t *testing.T) string {
	t.Helper()

	confDir := t.TempDir()
	originalConfigDir := appsettings.NginxSettings.ConfigDir
	appsettings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		appsettings.NginxSettings.ConfigDir = originalConfigDir
	})

	return confDir
}

func requireMCPRestrictedDirectiveError(t *testing.T, err error, wantParam string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected restricted directive error")
	}

	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("expected cosy error, got %v", err)
	}

	var wantErr *cosy.Error
	if !errors.As(internalconfig.ErrConfigDirectiveNotAllowed, &wantErr) {
		t.Fatalf("ErrConfigDirectiveNotAllowed is not a cosy error")
	}

	if cosyErr.Scope != wantErr.Scope || cosyErr.Code != wantErr.Code {
		t.Fatalf("expected cosy error %s:%d, got %s:%d", wantErr.Scope, wantErr.Code, cosyErr.Scope, cosyErr.Code)
	}

	if len(cosyErr.Params) != 1 || cosyErr.Params[0] != wantParam {
		t.Fatalf("expected params [%q], got %v", wantParam, cosyErr.Params)
	}
}
