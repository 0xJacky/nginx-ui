package site

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	internalconfig "github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/uozi-tech/cosy"
)

// Save must apply the directive denylist with nginx's token boundaries:
// restricted directives hidden behind characters that nginx keeps inside a
// token are rejected and never written, while ordinary comments, quoted
// hashes and ${var} references keep saving.
func TestSaveRejectsHiddenRestrictedDirectives(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantParam string
	}{
		{"hash mid-token", "server {\n    listen 80;\n    location / { add_header X a#; js_import m from conf.d/m.js; js_content m.main; }\n}\n", "js_import"},
		{"brace mid-token", "server {\n    listen 80;\n    add_header X a}#; js_content m.main;\n}\n", "js_content"},
		{"dollar brace", "server {\n    listen 80;\n    set $v ${host}#; perl_set $x 'sub { return 1; }';\n}\n", "perl_set"},
		{"comment past CR", "server {\n    listen 80; # note\r\"\n    js_import m.js; #\"\n}\n", "js_import"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confDir, _ := setupSiteMutationTest(t)

			err := Save(context.Background(), "example.com", tt.content, true, 0, nil, "")
			requireSiteRestrictedDirectiveError(t, err, tt.wantParam)

			path := filepath.Join(confDir, "sites-available", "example.com")
			if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("expected rejected Save to leave %q absent, stat error: %v", path, statErr)
			}
		})
	}
}

func TestSaveKeepsCommentsHashesAndVariables(t *testing.T) {
	confDir, waitForSyncQuery := setupSiteMutationTest(t)

	content := "# js_import m.js; comments may mention restricted directives\n" +
		"server {\n" +
		"    listen 80;# user root;\n" +
		"    server_name example.com;\n" +
		"    add_header X-Color \"#fff\";\n" +
		"    set $target ${scheme}://${host}#top;\n" +
		"    location ~ ^/a#b$ { return 302 https://example.com/#/home; }\n" +
		"    location ~ \"^/(?<id>[0-9]{1,3})$\" { return 204; }\n" +
		"}\n"
	if err := Save(context.Background(), "example.com", content, true, 0, nil, ""); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	waitForSyncQuery()

	written, err := os.ReadFile(filepath.Join(confDir, "sites-available", "example.com"))
	if err != nil {
		t.Fatalf("expected saved site file: %v", err)
	}
	if string(written) != content {
		t.Fatalf("saved content changed: %q", written)
	}
}

func requireSiteRestrictedDirectiveError(t *testing.T, err error, wantParam string) {
	t.Helper()

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
