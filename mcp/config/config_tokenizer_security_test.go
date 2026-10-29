package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// Restricted directives hidden behind characters that nginx keeps inside a
// token ('#' or '}' mid-token, '{' after '$', a CR inside a comment) must be
// rejected by the MCP write tools before anything reaches the disk.
var mcpHiddenDirectiveCases = []struct {
	name      string
	content   string
	wantParam string
}{
	{"hash mid-token", "server { listen 80; location / { add_header X a#; js_import m from conf.d/m.js; js_content m.main; } }\n", "js_import"},
	{"hash mid-token main context", "env A#; load_module modules/ngx_http_js_module.so; user root;\nevents {}\n", "load_module"},
	{"hash mid-token root workers", "env A#; user root;\nevents {}\n", "user root"},
	{"brace mid-token", "server { add_header X a}#; js_content m.main; }\n", "js_content"},
	{"dollar brace", "server { set $v ${host}#; perl_set $x 'sub { return 1; }'; }\n", "perl_set"},
	{"comment past CR", "# note\r\"\nlua_package_path \"/tmp/?.lua;;\"; #\"\n", "lua_package_path"},
}

func TestNginxConfigAddRejectsHiddenRestrictedDirectives(t *testing.T) {
	for _, tc := range mcpHiddenDirectiveCases {
		t.Run(tc.name, func(t *testing.T) {
			confDir := setupMCPConfigValidationTest(t)

			_, err := handleNginxConfigAdd(context.Background(), mcpgo.CallToolRequest{
				Params: mcpgo.CallToolParams{
					Arguments: map[string]any{"name": "app.conf", "content": tc.content},
				},
			})
			requireMCPRestrictedDirectiveError(t, err, tc.wantParam)

			path := filepath.Join(confDir, "app.conf")
			if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("expected rejected MCP add to leave %q absent, stat error: %v", path, statErr)
			}
		})
	}
}

func TestNginxConfigModifyRejectsHiddenRestrictedDirectives(t *testing.T) {
	for _, tc := range mcpHiddenDirectiveCases {
		t.Run(tc.name, func(t *testing.T) {
			confDir := setupMCPConfigValidationTest(t)
			path := filepath.Join(confDir, "app.conf")
			originalContent := []byte("server { listen 80; }\n")
			if err := os.WriteFile(path, originalContent, 0o644); err != nil {
				t.Fatalf("failed to seed config file: %v", err)
			}

			_, err := handleNginxConfigModify(context.Background(), mcpgo.CallToolRequest{
				Params: mcpgo.CallToolParams{
					Arguments: map[string]any{"relative_path": "app.conf", "content": tc.content},
				},
			})
			requireMCPRestrictedDirectiveError(t, err, tc.wantParam)

			content, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("failed to read config file: %v", readErr)
			}
			if string(content) != string(originalContent) {
				t.Fatalf("expected rejected MCP modify to keep original content %q, got %q", originalContent, content)
			}
		})
	}
}
