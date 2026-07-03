package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

func TestValidateConfigFilename(t *testing.T) {
	confDir := t.TempDir()
	for _, dir := range []string{
		"conf.d",
		"snippets",
		"sites-available",
		"sites-enabled",
		"streams-available",
		"streams-enabled",
	} {
		if err := os.MkdirAll(filepath.Join(confDir, dir), 0o755); err != nil {
			t.Fatalf("failed to create %s: %v", dir, err)
		}
	}

	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name: "allow root nginx conf",
			path: filepath.Join(confDir, "nginx.conf"),
		},
		{
			name: "allow standard root text file",
			path: filepath.Join(confDir, "mime.types"),
		},
		{
			name: "allow conf file anywhere",
			path: filepath.Join(confDir, "conf.d", "app.conf"),
		},
		{
			name: "allow site hostname",
			path: filepath.Join(confDir, "sites-available", "example.com"),
		},
		{
			name: "allow stream bare name",
			path: filepath.Join(confDir, "streams-enabled", "tcp_proxy"),
		},
		{
			name:    "reject shared library",
			path:    filepath.Join(confDir, "evil.so"),
			wantErr: true,
		},
		{
			name:    "reject non-conf bare name outside managed dirs",
			path:    filepath.Join(confDir, "conf.d", "evil"),
			wantErr: true,
		},
		{
			name:    "reject dangerous managed extension",
			path:    filepath.Join(confDir, "sites-available", "evil.pl"),
			wantErr: true,
		},
		{
			name:    "reject dangerous snippet extension",
			path:    filepath.Join(confDir, "snippets", "evil.pl"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfigFilename(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateConfigFilename(%q) expected error", tt.path)
				}
				var cosyErr *cosy.Error
				if !errors.As(err, &cosyErr) {
					t.Fatalf("ValidateConfigFilename(%q) expected cosy error, got %v", tt.path, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateConfigFilename(%q) unexpected error: %v", tt.path, err)
			}
		})
	}
}

func TestValidateConfigContentBytes(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		wantErr bool
	}{
		{
			name:    "allow nginx text",
			content: []byte("server {\n\tlisten 80;\n}\n"),
		},
		{
			name:    "reject invalid utf8",
			content: []byte{0xff, 0xfe, 0xfd},
			wantErr: true,
		},
		{
			name:    "reject null byte",
			content: []byte("server {\x00}\n"),
			wantErr: true,
		},
		{
			name:    "reject control byte",
			content: []byte("server {\x01}\n"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfigContentBytes(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateConfigContentBytes(%q) expected error", tt.content)
				}
				var cosyErr *cosy.Error
				if !errors.As(err, &cosyErr) {
					t.Fatalf("ValidateConfigContentBytes(%q) expected cosy error, got %v", tt.content, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateConfigContentBytes(%q) unexpected error: %v", tt.content, err)
			}
		})
	}
}

func TestValidateConfigDirectivesRejectsStatementSeparatedRestrictedDirectives(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantParam string
	}{
		{
			name:      "reject same-line lua package path after safe directive",
			content:   `server { listen 80; lua_package_path "/tmp/?.lua;;"; }`,
			wantParam: "lua_package_path",
		},
		{
			name:      "reject same-line njs import after safe directive",
			content:   "server { listen 80; js_import app.js; }\n",
			wantParam: "js_import",
		},
		{
			name:      "reject njs import after comment and whitespace",
			content:   "server {\n    listen 80; # safe listener\n \t js_import app.js;\n}\n",
			wantParam: "js_import",
		},
		{
			name:      "reject escaped njs directive name",
			content:   `server { listen 80; js\_import app.js; }`,
			wantParam: "js_import",
		},
		{
			name:      "reject root worker after same-line safe directive",
			content:   "worker_processes auto; user root;\nevents {}\n",
			wantParam: "user root",
		},
		{
			name:      "reject restricted module after same-line safe directive",
			content:   "pid nginx.pid; load_module modules/ngx_http_js_module.so;\nevents {}\n",
			wantParam: "load_module",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfigContent(tt.content)
			requireRestrictedDirectiveError(t, err, tt.wantParam)
		})
	}
}

func TestValidateConfigDirectivesAllowsSafeStatementsAndQuotedSemicolons(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "allow safe same-line config",
			content: `server { listen 80; location / { proxy_pass http://127.0.0.1:8080; } }
`,
		},
		{
			name:    "allow restricted directive text inside quotes",
			content: `server { listen 80; add_header X-Test "safe; js_import app.js;"; }`,
		},
		{
			name:    "allow escaped semicolon inside safe directive argument",
			content: `server { listen 80; add_header X-Test safe\;js_import; }`,
		},
		{
			name: "allow restricted directive text inside comments",
			content: `server {
    listen 80; # js_import app.js;
}
`,
		},
		{
			name: "allow existing lua block directives",
			content: `server {
    listen 443 ssl;
    ssl_certificate_by_lua_block {
        auto_ssl:ssl_certificate()
    }
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateConfigContent(tt.content); err != nil {
				t.Fatalf("ValidateConfigContent(%q) unexpected error: %v", tt.content, err)
			}
		})
	}
}

func requireRestrictedDirectiveError(t *testing.T, err error, wantParam string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected restricted directive error")
	}

	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("expected cosy error, got %v", err)
	}

	var wantErr *cosy.Error
	if !errors.As(ErrConfigDirectiveNotAllowed, &wantErr) {
		t.Fatalf("ErrConfigDirectiveNotAllowed is not a cosy error")
	}

	if cosyErr.Scope != wantErr.Scope || cosyErr.Code != wantErr.Code {
		t.Fatalf("expected cosy error %s:%d, got %s:%d", wantErr.Scope, wantErr.Code, cosyErr.Scope, cosyErr.Code)
	}

	if len(cosyErr.Params) != 1 || cosyErr.Params[0] != wantParam {
		t.Fatalf("expected params [%q], got %v", wantParam, cosyErr.Params)
	}
}
