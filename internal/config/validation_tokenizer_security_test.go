package config

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/template"
	"github.com/0xJacky/Nginx-UI/settings"
	templ "github.com/0xJacky/Nginx-UI/template"
)

// The directive denylist must split the configuration into the same tokens
// as nginx's ngx_conf_read_token. Wherever the two disagree, a restricted
// statement can be hidden from the validator while nginx still runs it.

type tokenizerAttackCase struct {
	name      string
	content   string
	wantParam string
}

var tokenizerAttackCases = []tokenizerAttackCase{
	// '#' is a literal character inside an unquoted token; ';' still ends
	// the statement, so the statements after it are live.
	{"hash mid-token hides load_module njs", "env A#; load_module modules/ngx_http_js_module.so;\nevents {}\n", "load_module"},
	{"hash mid-token hides load_module perl", "env A#; load_module modules/ngx_http_perl_module.so;\nevents {}\n", "load_module"},
	{"hash mid-token hides user root", "env A#; user root;\nevents {}\n", "user root"},
	{"hash mid-token hides njs import", "server { location / { add_header X a#; js_import m from conf.d/m.js; js_content m.main; } }\n", "js_import"},
	{"hash mid-token hides perl_set", "server { set $v a#; perl_set $x 'sub { return 1; }'; }\n", "perl_set"},
	{"hash mid-token hides lua_package_path", "server { set $v a#; lua_package_path \"/tmp/?.lua;;\"; }\n", "lua_package_path"},
	{"hash after variable", "server { set $v $host#; js_content m.main; }\n", "js_content"},
	{"hash with suffix mid-token", "server { add_header X-A a#b; js_import m.js; }\n", "js_import"},
	{"hash mid-token before tab", "server {\n\tset $v a#;\tjs_import m.js;\n}\n", "js_import"},
	{"hash mid-token with CRLF", "server {\r\n    set $v a#; js_import m.js;\r\n}\r\n", "js_import"},
	{"hash after non-breaking space", "env A #; load_module modules/ngx_http_js_module.so;\nevents {}\n", "load_module"},
	{"hash after mid-token quote", "server { set $v a'#; js_import m.js; #'\n}\n", "js_import"},
	{"hash after escaped backslash", "server { set $v a\\\\#; js_import m.js; }\n", "js_import"},
	{"hash after escaped hash at token start", "server { add_header X \\##; js_import m.js; }\n", "js_import"},
	{"hash after escaped semicolon", "server { add_header X a\\;#; js_import m.js; }\n", "js_import"},

	// '}' is a literal character inside an unquoted token.
	{"brace mid-token njs import", "server { js_import a}b.js; }\n", "js_import"},
	{"brace then hash mid-token", "server { add_header X a}#; js_content m.main; }\n", "js_content"},
	{"brace then quote mid-token", "server { add_header X a}\"; js_import m.js; #\"\n}\n", "js_import"},

	// '{' right after '$' stays in the token (the ${name} form).
	{"dollar-brace then hash", "server { set $v ${host}#; js_content m.main; }\n", "js_content"},
	{"dollar-brace then quote", "server { default_type ${\"; js_import m.js; #\"\n}\n", "js_import"},
	{"dollar-brace in directive position", "${a}#; js_import m.js;\n", "js_import"},

	// Comments end at LF only; a bare CR is ordinary whitespace inside them.
	{"comment continues past CR", "# note\r\"\njs_import m.js; #\"\n", "js_import"},

	// A statement still pending when '}' arrives is checked; nginx rejects
	// that form, so checking it can only reject configs nginx refuses too.
	{"statement before closing brace", "server { js_import m.js }\n", "js_import"},
}

type tokenizerLegitCase struct {
	name    string
	content string
}

var tokenizerLegitCases = []tokenizerLegitCase{
	{"full-line comment mentioning restricted directive", "# js_import m.js; load_module modules/ngx_http_js_module.so;\nserver { listen 80; }\n"},
	{"comment after whitespace", "server {\n    listen 80; # js_import app.js;\n}\n"},
	{"comment straight after semicolon", "server { listen 80;# js_import app.js;\n}\n"},
	{"comment straight after open brace", "server {# js_import app.js;\n    listen 80;\n}\n"},
	{"comment straight after close brace", "server { listen 80; }# js_import app.js;\n"},
	{"comment after tab", "server {\n\tlisten 80;\t# user root;\n}\n"},
	{"comment with CRLF line endings", "server {\r\n    listen 80; # js_import app.js;\r\n}\r\n"},
	{"comment containing CR before LF", "# user root;\r more comment js_import a.js;\nserver { listen 80; }\n"},
	{"hash inside double quotes", "server { add_header X \"a#b; js_import m.js;\"; }\n"},
	{"hash inside single quotes", "server { add_header X 'a#b; js_import m.js;'; }\n"},
	{"escaped quote then hash inside quotes", "server { add_header X \"a\\\"#b\"; }\n"},
	{"color value in quotes", "server { add_header X-Color \"#fff\"; }\n"},
	{"quoted URL with fragment", "server { return 301 \"https://example.com/#/home\"; }\n"},
	{"unquoted URL with fragment", "server { return 302 https://example.com/app#/home; }\n"},
	{"proxy_pass then fragment redirect", "server { location / { proxy_pass http://127.0.0.1:8080; } location /a { return 302 /b#c; } }\n"},
	{"variable followed by hash", "server { set $key $host#$uri; }\n"},
	{"dollar-brace variables", "server { set $a \"${host}_x\"; set $b ${host}suffix; return 200 ${scheme}://${host}; }\n"},
	{"regex location with hash", "server { location ~ ^/a#b$ { return 204; } }\n"},
	{"quoted regex location with braces", "server { location ~ \"^/(?<id>[0-9]{1,3})$\" { return 204; } }\n"},
	{"regex location with closing brace mid-token", "server { location ~ ^/x}y$ { return 204; } }\n"},
	{"rewrite with hash", "server { rewrite ^/old#(.*)$ /new/$1 last; }\n"},
	{"brace mid-token value", "server { add_header X-Brace a}b; }\n"},
	{"log_format with hash", "http { log_format main '$remote_addr #$status'; }\n"},
	{"map block", "http {\n    map $http_upgrade $connection_upgrade {\n        default upgrade;\n        '' close;\n    }\n    map $uri $flag { ~^/a#b 1; default 0; }\n}\n"},
	{"if blocks", "server {\n    if ($request_uri ~* \"#\") { return 403; }\n    if ($host = \"a\") { return 404; }\n    if ($http_x) { return 410; }\n}\n"},
	{"try_files", "server { location / { try_files $uri $uri/ /index.php?$args; } }\n"},
	{"escaped space", "server { add_header X a\\ b; }\n"},
	{"existing quoted and escaped semicolons", "server { listen 80; add_header X-Test \"safe; js_import app.js;\"; add_header X-Test safe\\;js_import; }\n"},
	{"lua block directives", "server {\n    listen 443 ssl;\n    ssl_certificate_by_lua_block {\n        auto_ssl:ssl_certificate()\n    }\n    location / {\n        access_by_lua_block {\n            if ngx.var.x == \"#\" then return ngx.exit(403) end\n            local t = {1, 2}\n        }\n        content_by_lua_block { ngx.say(\"#hash\") }\n    }\n}\n"},
	{"user and load_module that are not restricted", "user www-data;\nload_module modules/ngx_stream_module.so;\nevents {}\n"},
}

func tokenizerEntryPoints(confDir string) map[string]func(string) error {
	path := filepath.Join(confDir, "conf.d", "app.conf")
	return map[string]func(string) error{
		"ValidateConfigContent":      ValidateConfigContent,
		"ValidateConfigContentBytes": func(content string) error { return ValidateConfigContentBytes([]byte(content)) },
		"ValidateConfigDirectives":   func(content string) error { return ValidateConfigDirectives([]byte(content)) },
		"ValidateConfigFile":         func(content string) error { return ValidateConfigFile(path, content) },
		"ValidateConfigFileBytes":    func(content string) error { return ValidateConfigFileBytes(path, []byte(content)) },
	}
}

func setupTokenizerConfDir(t *testing.T) string {
	t.Helper()
	confDir := t.TempDir()
	previous := *settings.NginxSettings
	settings.NginxSettings.ConfigDir = confDir
	settings.NginxSettings.ConfigPath = filepath.Join(confDir, "nginx.conf")
	t.Cleanup(func() { *settings.NginxSettings = previous })
	for _, dir := range []string{"conf.d", "sites-available", "streams-available"} {
		if err := os.MkdirAll(filepath.Join(confDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return confDir
}

func TestTokenizerRejectsHiddenRestrictedDirectives(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	for entryName, entry := range tokenizerEntryPoints(confDir) {
		for _, tc := range tokenizerAttackCases {
			t.Run(entryName+"/"+tc.name, func(t *testing.T) {
				requireRestrictedDirectiveError(t, entry(tc.content), tc.wantParam)
			})
		}
	}
}

func TestTokenizerRejectsHiddenDirectivesInIncludedFiles(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "conf.d", "app.conf")
	for _, tc := range tokenizerAttackCases {
		t.Run(tc.name, func(t *testing.T) {
			included := filepath.Join(confDir, "conf.d", "included.inc")
			writeIncludeTestFile(t, included, tc.content)
			requireRestrictedDirectiveError(t, ValidateConfigFile(path, "include "+included+";\n"), tc.wantParam)
		})
	}
}

func TestTokenizerFollowsIncludesBehindLiteralCharacters(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	path := filepath.Join(confDir, "conf.d", "app.conf")
	included := filepath.Join(confDir, "conf.d", "inner.inc")
	writeIncludeTestFile(t, included, "js_import m from conf.d/m.js;\n")

	for _, prefix := range []string{
		"set $v a#;",
		"add_header X a}#;",
		"set $v ${host}#;",
		"env A #;",
	} {
		t.Run(prefix, func(t *testing.T) {
			content := "server { " + prefix + " include " + included + "; }\n"
			requireRestrictedDirectiveError(t, ValidateConfigFile(path, content), "js_import")
			requireRestrictedDirectiveError(t, ValidateConfigFileBytes(path, []byte(content)), "js_import")
		})
	}
}

func TestTokenizerKeepsLegitimateConfigValid(t *testing.T) {
	confDir := setupTokenizerConfDir(t)
	for entryName, entry := range tokenizerEntryPoints(confDir) {
		for _, tc := range tokenizerLegitCases {
			t.Run(entryName+"/"+tc.name, func(t *testing.T) {
				if err := entry(tc.content); err != nil {
					t.Fatalf("%s(%q) unexpected error: %v", entryName, tc.content, err)
				}
			})
		}
	}
}

func TestTokenizerKeepsRepositoryFixturesValid(t *testing.T) {
	setupTokenizerConfDir(t)

	repoRoot := filepath.Join("..", "..")
	var fixtures []string
	for _, pattern := range []string{
		"internal/ingress/testdata/render/*.conf",
		"internal/nginx/conf/*.conf",
		"internal/self_check/test_cases/*.conf",
		"resources/demo/*.conf",
		"resources/docker/*.conf",
		"resources/docker/tests/fixtures/*.conf",
	} {
		matches, err := filepath.Glob(filepath.Join(repoRoot, pattern))
		if err != nil {
			t.Fatal(err)
		}
		fixtures = append(fixtures, matches...)
	}
	if len(fixtures) == 0 {
		t.Fatal("no repository fixtures found")
	}
	// The demo image ships its own main config with root workers; it is never
	// written through the UI, and the tokenizer must still see the directive.
	wantRestricted := map[string]string{"resources/demo/nginx.conf": "user root"}
	for _, fixture := range fixtures {
		relative, err := filepath.Rel(repoRoot, fixture)
		if err != nil {
			t.Fatal(err)
		}
		relative = filepath.ToSlash(relative)
		t.Run(relative, func(t *testing.T) {
			content, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatal(err)
			}
			err = ValidateConfigContentBytes(content)
			if wantParam, ok := wantRestricted[relative]; ok {
				requireRestrictedDirectiveError(t, err, wantParam)
				return
			}
			if err != nil {
				t.Fatalf("fixture rejected: %v", err)
			}
		})
	}
}

func TestTokenizerKeepsBuiltinTemplatesValid(t *testing.T) {
	setupTokenizerConfDir(t)

	count := 0
	err := fs.WalkDir(templ.DistFS, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(name, ".conf") {
			return err
		}
		count++
		t.Run(name, func(t *testing.T) {
			raw, err := templ.DistFS.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			body := string(raw)
			if _, after, found := strings.Cut(body, template.HeaderEnd); found {
				body = after
			}
			info := template.GetTemplateInfo(path.Dir(name), path.Base(name))
			rendered, err := template.RenderText(name, body, info.Variables)
			if err != nil {
				t.Fatalf("render template: %v", err)
			}
			if err := ValidateConfigContent(rendered); err != nil {
				t.Fatalf("rendered template rejected: %v\n%s", err, rendered)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no built-in templates found")
	}
}
