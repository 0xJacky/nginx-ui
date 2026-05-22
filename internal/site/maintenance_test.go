package site

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/tufanbarisyildirim/gonginx/parser"
	cSettings "github.com/uozi-tech/cosy/settings"
)

func TestCreateMaintenanceConfig_PreservesForwardedHost(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
	})

	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(`server {
    listen 80;
    server_name example.com;
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, "")

	if !strings.Contains(content, "proxy_set_header X-Forwarded-Host $http_host;") {
		t.Fatalf("maintenance config = %q, want forwarded host header preservation", content)
	}

	if !strings.Contains(content, "proxy_set_header X-Forwarded-Proto $scheme;") {
		t.Fatalf("maintenance config = %q, want forwarded proto header preservation", content)
	}

	if !strings.Contains(content, "proxy_pass http://127.0.0.1:9000;") {
		t.Fatalf("maintenance config = %q, want proxy to nginx-ui backend", content)
	}
}

func TestCreateMaintenanceConfig_PreservesTLSHandshakeDirectives(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	snippetsDir := filepath.Join(nginxConfigDir, "snippets")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}
	if err := os.MkdirAll(snippetsDir, 0755); err != nil {
		t.Fatalf("failed to create snippets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(snippetsDir, "ssl-options.conf"), []byte(`ssl_protocols TLSv1.2 TLSv1.3;
ssl_ciphers HIGH:!aNULL:!MD5;
location /unexpected {
    proxy_pass http://127.0.0.1:12345;
}
`), 0644); err != nil {
		t.Fatalf("failed to write ssl include: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(`server {
    listen 443 ssl;
    server_name example.com;
    include snippets/ssl-options.conf;
    include snippets/proxy.conf;
    ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;
    ssl_prefer_server_ciphers on;
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)

	expectedDirectives := []string{
		"ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;",
		"ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;",
		"ssl_prefer_server_ciphers on;",
		"ssl_protocols TLSv1.2 TLSv1.3;",
		"ssl_ciphers HIGH:!aNULL:!MD5;",
	}

	for _, expected := range expectedDirectives {
		if !strings.Contains(content, expected) {
			t.Fatalf("maintenance config = %q, want TLS directive %q", content, expected)
		}
	}

	unexpectedContent := []string{
		"include snippets/ssl-options.conf;",
		"include snippets/proxy.conf;",
		"proxy_pass http://127.0.0.1:12345;",
		"location /unexpected",
	}

	for _, unexpected := range unexpectedContent {
		if strings.Contains(content, unexpected) {
			t.Fatalf("maintenance config = %q, want to exclude %q", content, unexpected)
		}
	}
}

func TestCreateMaintenanceConfig_RecursivelyExpandsTLSIncludesAndSkipsCycles(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	snippetsDir := filepath.Join(nginxConfigDir, "snippets")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}
	if err := os.MkdirAll(snippetsDir, 0755); err != nil {
		t.Fatalf("failed to create snippets dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(snippetsDir, "a.conf"), []byte(`include b.conf;
ssl_session_cache shared:SSL:10m;
`), 0644); err != nil {
		t.Fatalf("failed to write a.conf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(snippetsDir, "b.conf"), []byte(`include a.conf;
ssl_session_timeout 10m;
`), 0644); err != nil {
		t.Fatalf("failed to write b.conf: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(`server {
    listen 443 ssl;
    server_name example.com;
    include snippets/a.conf;
    ssl_certificate /cert.pem;
    ssl_certificate_key /key.pem;
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)

	for _, expected := range []string{
		"ssl_session_cache shared:SSL:10m;",
		"ssl_session_timeout 10m;",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("maintenance config = %q, want recursive TLS directive %q", content, expected)
		}
	}
}

func TestCreateMaintenanceConfig_ExpandsNestedRelativeWildcardIncludes(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	snippetsDir := filepath.Join(nginxConfigDir, "snippets")
	tlsDir := filepath.Join(snippetsDir, "tls")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}
	if err := os.MkdirAll(tlsDir, 0755); err != nil {
		t.Fatalf("failed to create tls dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(snippetsDir, "base.conf"), []byte("include tls/*.conf;\n"), 0644); err != nil {
		t.Fatalf("failed to write base include: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tlsDir, "options.conf"), []byte("ssl_protocols TLSv1.2 TLSv1.3;\n"), 0644); err != nil {
		t.Fatalf("failed to write nested wildcard include: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(`server {
    listen 443 ssl;
    server_name example.com;
    include snippets/base.conf;
    ssl_certificate /cert.pem;
    ssl_certificate_key /key.pem;
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)
	if !strings.Contains(content, "ssl_protocols TLSv1.2 TLSv1.3;") {
		t.Fatalf("maintenance config = %q, want TLS directive from nested relative wildcard include", content)
	}
	if strings.Contains(content, "include tls/*.conf;") {
		t.Fatalf("maintenance config = %q, want nested wildcard include omitted", content)
	}
}

func TestCreateMaintenanceConfig_SkipsServerDirectivesFromIncludes(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	snippetsDir := filepath.Join(nginxConfigDir, "snippets")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}
	if err := os.MkdirAll(snippetsDir, 0755); err != nil {
		t.Fatalf("failed to create snippets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(snippetsDir, "common.conf"), []byte(`listen 8443 ssl;
server_name internal.example.com;
http2 on;
ssl_protocols TLSv1.3;
`), 0644); err != nil {
		t.Fatalf("failed to write common include: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(`server {
    listen 443 ssl;
    server_name example.com;
    include snippets/common.conf;
    ssl_certificate /cert.pem;
    ssl_certificate_key /key.pem;
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)
	for _, unexpected := range []string{
		"listen 8443 ssl;",
		"server_name internal.example.com;",
		"http2 on;",
	} {
		if strings.Contains(content, unexpected) {
			t.Fatalf("maintenance config = %q, want include server directive %q omitted", content, unexpected)
		}
	}
	if !strings.Contains(content, "ssl_protocols TLSv1.3;") {
		t.Fatalf("maintenance config = %q, want TLS directive from include", content)
	}
}

func TestCreateMaintenanceConfig_PreservesTLSBlockDirectives(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(`server {
    listen 443 ssl;
    server_name example.com;
    ssl_certificate_by_lua_block {
        auto_ssl:ssl_certificate()
    }
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)
	if !strings.Contains(content, "ssl_certificate_by_lua_block") || !strings.Contains(content, "auto_ssl:ssl_certificate()") {
		t.Fatalf("maintenance config = %q, want TLS block directive preserved", content)
	}
}

func TestCreateMaintenanceConfig_PreservesLuaBlockWithSemicolonsAndBraces(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	// Regression guard for Raw-field fidelity: the lua block contains both a
	// statement-terminator (;) and a nested {} literal that could trip a naive
	// dumper or scanner. The whole block must round-trip intact through Raw.
	p := parser.NewStringParser(`server {
    listen 443 ssl;
    server_name example.com;
    ssl_certificate_by_lua_block {
        local opts = { ttl = 3600 }
        ngx.log(ngx.INFO, "ttl=" .. opts.ttl .. "; ok")
    }
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)

	// Token-level invariants. The gonginx dumper may normalize whitespace inside
	// the lua block, so assert on tokens that must survive verbatim instead of
	// matching the exact source layout:
	//   - the lua-string semicolon "; ok" must NOT be treated as a directive
	//     terminator;
	//   - the nested lua table braces must NOT collapse the outer directive.
	for _, expected := range []string{
		"ssl_certificate_by_lua_block",
		`"; ok"`,
		"ttl = 3600",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("maintenance config = %q, want lua block fragment %q preserved", content, expected)
		}
	}

	// The outer block must still parse as a single nginx directive. If the lua
	// `{ ttl = 3600 }` were misread as nested nginx blocks we would see more
	// than one opening of the directive.
	if got := strings.Count(content, "ssl_certificate_by_lua_block {"); got != 1 {
		t.Fatalf("maintenance config = %q, want exactly one ssl_certificate_by_lua_block opener, got %d", content, got)
	}
}

func TestCreateMaintenanceConfig_PreservesQuotedTLSDirectiveParams(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(`server {
    listen 443 ssl;
    server_name example.com;
    ssl_certificate "/etc/nginx/certs/example cert.pem";
    ssl_certificate_key "/etc/nginx/certs/example key.pem";
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)
	for _, expected := range []string{
		`ssl_certificate "/etc/nginx/certs/example cert.pem";`,
		`ssl_certificate_key "/etc/nginx/certs/example key.pem";`,
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("maintenance config = %q, want quoted TLS directive %q", content, expected)
		}
	}
}

func TestCreateMaintenanceConfig_ExpandsWildcardTLSIncludesInSortedOrder(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	tlsDir := filepath.Join(nginxConfigDir, "snippets", "tls")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}
	if err := os.MkdirAll(tlsDir, 0755); err != nil {
		t.Fatalf("failed to create tls dir: %v", err)
	}

	files := map[string]string{
		"b.conf": "ssl_ciphers HIGH:!aNULL:!MD5;\n",
		"a.conf": "ssl_protocols TLSv1.2 TLSv1.3;\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tlsDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(`server {
    listen 443 ssl;
    server_name example.com;
    include snippets/tls/*.conf;
    ssl_certificate /cert.pem;
    ssl_certificate_key /key.pem;
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)

	protocolsIndex := strings.Index(content, "ssl_protocols TLSv1.2 TLSv1.3;")
	ciphersIndex := strings.Index(content, "ssl_ciphers HIGH:!aNULL:!MD5;")
	if protocolsIndex == -1 || ciphersIndex == -1 {
		t.Fatalf("maintenance config = %q, want wildcard TLS directives", content)
	}
	if protocolsIndex > ciphersIndex {
		t.Fatalf("maintenance config = %q, want wildcard files expanded in sorted order", content)
	}
	if strings.Contains(content, "include snippets/tls/*.conf;") {
		t.Fatalf("maintenance config = %q, want wildcard include omitted", content)
	}
}

func TestCreateMaintenanceConfig_SkipsWildcardIncludesOutsideNginxConfigDir(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	outsideDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideDir, "ssl.conf"), []byte("ssl_ciphers HIGH;\n"), 0644); err != nil {
		t.Fatalf("failed to write outside include: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(fmt.Sprintf(`server {
    listen 443 ssl;
    server_name example.com;
    include %s;
    ssl_certificate /cert.pem;
    ssl_certificate_key /key.pem;
}`, filepath.ToSlash(filepath.Join(outsideDir, "*.conf"))), parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)
	if strings.Contains(content, "ssl_ciphers HIGH;") {
		t.Fatalf("maintenance config = %q, want to skip wildcard outside nginx config dir", content)
	}
}

func TestCreateMaintenanceConfig_LimitsWildcardIncludeMatches(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	tlsDir := filepath.Join(nginxConfigDir, "snippets", "tls")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}
	if err := os.MkdirAll(tlsDir, 0755); err != nil {
		t.Fatalf("failed to create tls dir: %v", err)
	}

	for i := 0; i < maintenanceMaxWildcardMatches+1; i++ {
		name := fmt.Sprintf("%02d.conf", i)
		content := fmt.Sprintf("ssl_conf_command Options%d Value%d;\n", i, i)
		if err := os.WriteFile(filepath.Join(tlsDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(`server {
    listen 443 ssl;
    server_name example.com;
    include snippets/tls/*.conf;
    ssl_certificate /cert.pem;
    ssl_certificate_key /key.pem;
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)
	if !strings.Contains(content, "ssl_conf_command Options31 Value31;") {
		t.Fatalf("maintenance config = %q, want last allowed wildcard match", content)
	}
	if strings.Contains(content, "ssl_conf_command Options32 Value32;") {
		t.Fatalf("maintenance config = %q, want wildcard matches capped at %d", content, maintenanceMaxWildcardMatches)
	}
}

func TestCreateMaintenanceConfig_LimitsWildcardIncludeMatchesAfterFiltering(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	outsideDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	tlsDir := filepath.Join(nginxConfigDir, "snippets", "tls")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}
	if err := os.MkdirAll(tlsDir, 0755); err != nil {
		t.Fatalf("failed to create tls dir: %v", err)
	}

	outsideFile := filepath.Join(outsideDir, "outside.conf")
	if err := os.WriteFile(outsideFile, []byte("ssl_ciphers EVIL;\n"), 0644); err != nil {
		t.Fatalf("failed to write outside file: %v", err)
	}
	for i := 0; i < maintenanceMaxWildcardMatches; i++ {
		linkPath := filepath.Join(tlsDir, fmt.Sprintf("%02d-escape.conf", i))
		if err := os.Symlink(outsideFile, linkPath); err != nil {
			t.Skipf("symlink creation is not available in this environment: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(tlsDir, "zz-real.conf"), []byte("ssl_protocols TLSv1.2 TLSv1.3;\n"), 0644); err != nil {
		t.Fatalf("failed to write legal wildcard include: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(`server {
    listen 443 ssl;
    server_name example.com;
    include snippets/tls/*.conf;
    ssl_certificate /cert.pem;
    ssl_certificate_key /key.pem;
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)
	if !strings.Contains(content, "ssl_protocols TLSv1.2 TLSv1.3;") {
		t.Fatalf("maintenance config = %q, want legal wildcard match after filtering invalid matches", content)
	}
	if strings.Contains(content, "ssl_ciphers EVIL;") {
		t.Fatalf("maintenance config = %q, want symlink escape still filtered", content)
	}
}

func TestCreateMaintenanceConfig_RejectsWildcardSymlinkEscape(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	outsideDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	tlsDir := filepath.Join(nginxConfigDir, "snippets", "tls")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}
	if err := os.MkdirAll(tlsDir, 0755); err != nil {
		t.Fatalf("failed to create tls dir: %v", err)
	}
	outsideFile := filepath.Join(outsideDir, "evil.conf")
	if err := os.WriteFile(outsideFile, []byte("ssl_ciphers EVIL;\n"), 0644); err != nil {
		t.Fatalf("failed to write outside file: %v", err)
	}
	linkPath := filepath.Join(tlsDir, "ssl-escape.conf")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Skipf("symlink creation is not available in this environment: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(`server {
    listen 443 ssl;
    server_name example.com;
    include snippets/tls/*.conf;
    ssl_certificate /cert.pem;
    ssl_certificate_key /key.pem;
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)
	if strings.Contains(content, "ssl_ciphers EVIL;") {
		t.Fatalf("maintenance config = %q, want to reject wildcard symlink escape", content)
	}
}

func TestMaintenanceIncludeExpander_AllowsCertbotOptionsPath(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	originalCertbotPath := certbotNginxTLSOptionsPath
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		certbotNginxTLSOptionsPath = originalCertbotPath
	})

	settings.NginxSettings.ConfigDir = t.TempDir()
	certbotDir := t.TempDir()
	certbotNginxTLSOptionsPath = filepath.Join(certbotDir, "options-ssl-nginx.conf")
	if err := os.WriteFile(certbotNginxTLSOptionsPath, []byte("ssl_protocols TLSv1.3;\n"), 0644); err != nil {
		t.Fatalf("failed to write certbot options path: %v", err)
	}

	expander := newMaintenanceIncludeExpander("")

	if !expander.isAllowedSingleInclude(certbotNginxTLSOptionsPath) {
		t.Fatalf("expected certbot options path %q to be allowed", certbotNginxTLSOptionsPath)
	}

	if expander.isAllowedSingleInclude("/etc/letsencrypt/other.conf") {
		t.Fatalf("expected unrelated path under /etc/letsencrypt to be rejected")
	}

	if expander.isAllowedSingleInclude("/etc/passwd") {
		t.Fatalf("expected arbitrary system path to be rejected")
	}
}

func TestMaintenanceIncludeExpander_RejectsCertbotOptionsSymlink(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	originalCertbotPath := certbotNginxTLSOptionsPath
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		certbotNginxTLSOptionsPath = originalCertbotPath
	})

	settings.NginxSettings.ConfigDir = t.TempDir()
	certbotDir := t.TempDir()
	outsideDir := t.TempDir()
	certbotNginxTLSOptionsPath = filepath.Join(certbotDir, "options-ssl-nginx.conf")
	outsideFile := filepath.Join(outsideDir, "outside.conf")
	if err := os.WriteFile(outsideFile, []byte("ssl_ciphers EVIL;\n"), 0644); err != nil {
		t.Fatalf("failed to write outside file: %v", err)
	}
	if err := os.Symlink(outsideFile, certbotNginxTLSOptionsPath); err != nil {
		t.Skipf("symlink creation is not available in this environment: %v", err)
	}

	expander := newMaintenanceIncludeExpander("")
	if expander.isAllowedSingleInclude(certbotNginxTLSOptionsPath) {
		t.Fatalf("expected certbot options symlink to be rejected")
	}
}

func TestMaintenanceIncludeExpander_UsesBaseDirUnderConfigDir(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	expander := newMaintenanceIncludeExpander(sitesAvailableDir)

	if expander.baseDir != filepath.Clean(sitesAvailableDir) {
		t.Fatalf("baseDir = %q, want %q", expander.baseDir, filepath.Clean(sitesAvailableDir))
	}
}

func TestMaintenanceIncludeExpander_FallsBackWhenBaseDirEscapesConfigDir(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	outsideDir := t.TempDir()
	settings.NginxSettings.ConfigDir = nginxConfigDir

	expander := newMaintenanceIncludeExpander(outsideDir)

	if expander.baseDir != filepath.Clean(nginxConfigDir) {
		t.Fatalf("baseDir = %q, want fallback to confDir %q", expander.baseDir, filepath.Clean(nginxConfigDir))
	}
}

func TestMaintenanceIncludeExpander_ResolveIncludePathSkipsExistingRelativeEscape(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	outsideDir := t.TempDir()
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}
	outsideFile := filepath.Join(outsideDir, "ssl.conf")
	if err := os.WriteFile(outsideFile, []byte("ssl_ciphers EVIL;\n"), 0644); err != nil {
		t.Fatalf("failed to write outside file: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	expander := newMaintenanceIncludeExpander(sitesAvailableDir)
	includePath, err := filepath.Rel(sitesAvailableDir, outsideFile)
	if err != nil {
		t.Fatalf("failed to build relative escape include path: %v", err)
	}

	resolvedPath := expander.resolveIncludePath(includePath)
	if filepath.Clean(resolvedPath) == filepath.Clean(outsideFile) {
		t.Fatalf("resolveIncludePath(%q) = %q, want existing path outside nginx config dir ignored", includePath, resolvedPath)
	}
}

func TestMaintenanceIncludeExpander_ResolveIncludePathFallsBackToConfDirOnEscape(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	expander := newMaintenanceIncludeExpander(sitesAvailableDir)

	resolvedPath := expander.resolveIncludePath(filepath.Join("..", "..", "escape.conf"))
	if resolvedPath != filepath.Clean(nginxConfigDir) {
		t.Fatalf("resolveIncludePath escape fallback = %q, want confDir %q", resolvedPath, filepath.Clean(nginxConfigDir))
	}
}

func TestMaintenanceIncludeExpander_ResolveWildcardIncludePathRejectsAbsoluteEscape(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	outsideDir := t.TempDir()
	settings.NginxSettings.ConfigDir = nginxConfigDir
	expander := newMaintenanceIncludeExpander("")

	outsidePattern := filepath.Join(outsideDir, "*.conf")
	resolvedPath := expander.resolveWildcardIncludePath(outsidePattern)
	if filepath.Clean(resolvedPath) == filepath.Clean(outsidePattern) {
		t.Fatalf("resolveWildcardIncludePath(%q) = %q, want absolute path outside nginx config dir rejected", outsidePattern, resolvedPath)
	}
}

func TestMaintenanceIncludeExpander_ExtractIncludeFileRejectsDisallowedPath(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	settings.NginxSettings.ConfigDir = t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "ssl.conf")
	if err := os.WriteFile(outsideFile, []byte("ssl_ciphers EVIL;\n"), 0644); err != nil {
		t.Fatalf("failed to write outside include: %v", err)
	}

	expander := newMaintenanceIncludeExpander("")
	if directives := expander.extractIncludeFile(outsideFile, 1); len(directives) != 0 {
		t.Fatalf("extractIncludeFile(%q) returned %d directives, want disallowed path rejected", outsideFile, len(directives))
	}
}

func TestCreateMaintenanceConfig_EnforcesIncludeDepthLimit(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	snippetsDir := filepath.Join(nginxConfigDir, "snippets")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}
	if err := os.MkdirAll(snippetsDir, 0755); err != nil {
		t.Fatalf("failed to create snippets dir: %v", err)
	}

	// Build a chain s1 -> s2 -> ... -> s7, each carrying a unique ssl_conf_command.
	// Top-level include into s1 starts at depth 1, so the chain must exceed
	// maintenanceMaxIncludeDepth + 1 levels to ensure the last link is dropped.
	chainLen := maintenanceMaxIncludeDepth + 2
	for i := 1; i <= chainLen; i++ {
		var body strings.Builder
		body.WriteString(fmt.Sprintf("ssl_conf_command Level%d Value%d;\n", i, i))
		if i < chainLen {
			body.WriteString(fmt.Sprintf("include s%d.conf;\n", i+1))
		}
		path := filepath.Join(snippetsDir, fmt.Sprintf("s%d.conf", i))
		if err := os.WriteFile(path, []byte(body.String()), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(`server {
    listen 443 ssl;
    server_name example.com;
    include snippets/s1.conf;
    ssl_certificate /cert.pem;
    ssl_certificate_key /key.pem;
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)

	// Levels 1..maintenanceMaxIncludeDepth must be present.
	for i := 1; i <= maintenanceMaxIncludeDepth; i++ {
		expected := fmt.Sprintf("ssl_conf_command Level%d Value%d;", i, i)
		if !strings.Contains(content, expected) {
			t.Fatalf("maintenance config = %q, want directive from depth %d (%q)", content, i, expected)
		}
	}

	// Anything beyond the depth limit must be dropped.
	for i := maintenanceMaxIncludeDepth + 1; i <= chainLen; i++ {
		unexpected := fmt.Sprintf("ssl_conf_command Level%d Value%d;", i, i)
		if strings.Contains(content, unexpected) {
			t.Fatalf("maintenance config = %q, want directive at depth %d to be dropped (%q)", content, i, unexpected)
		}
	}
}

func TestCreateMaintenanceConfig_AcceptsAbsoluteIncludeInsideConfigDir(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	snippetsDir := filepath.Join(nginxConfigDir, "snippets")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}
	if err := os.MkdirAll(snippetsDir, 0755); err != nil {
		t.Fatalf("failed to create snippets dir: %v", err)
	}

	absoluteInclude := filepath.Join(snippetsDir, "absolute-ssl.conf")
	if err := os.WriteFile(absoluteInclude, []byte("ssl_protocols TLSv1.3;\n"), 0644); err != nil {
		t.Fatalf("failed to write absolute include: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(fmt.Sprintf(`server {
    listen 443 ssl;
    server_name example.com;
    include %s;
    ssl_certificate /cert.pem;
    ssl_certificate_key /key.pem;
}`, filepath.ToSlash(absoluteInclude)), parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)

	if !strings.Contains(content, "ssl_protocols TLSv1.3;") {
		t.Fatalf("maintenance config = %q, want directive from absolute include under config dir", content)
	}
	if strings.Contains(content, "include "+filepath.ToSlash(absoluteInclude)) {
		t.Fatalf("maintenance config = %q, want absolute include directive itself to be omitted", content)
	}
}

func TestCreateMaintenanceConfig_IsolatesIncludeExpansionPerServer(t *testing.T) {
	originalPort := cSettings.ServerSettings.Port
	originalHTTPS := cSettings.ServerSettings.EnableHTTPS
	originalChallengePort := settings.CertSettings.HTTPChallengePort
	originalConfigDir := settings.NginxSettings.ConfigDir

	t.Cleanup(func() {
		cSettings.ServerSettings.Port = originalPort
		cSettings.ServerSettings.EnableHTTPS = originalHTTPS
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	nginxConfigDir := t.TempDir()
	sitesAvailableDir := filepath.Join(nginxConfigDir, "sites-available")
	snippetsDir := filepath.Join(nginxConfigDir, "snippets")
	if err := os.MkdirAll(sitesAvailableDir, 0755); err != nil {
		t.Fatalf("failed to create sites-available dir: %v", err)
	}
	if err := os.MkdirAll(snippetsDir, 0755); err != nil {
		t.Fatalf("failed to create snippets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(snippetsDir, "shared-ssl.conf"), []byte("ssl_protocols TLSv1.3;\n"), 0644); err != nil {
		t.Fatalf("failed to write shared include: %v", err)
	}

	settings.NginxSettings.ConfigDir = nginxConfigDir
	cSettings.ServerSettings.Port = 9000
	cSettings.ServerSettings.EnableHTTPS = false
	settings.CertSettings.HTTPChallengePort = "9180"

	p := parser.NewStringParser(`server {
    listen 443 ssl;
    server_name a.example.com;
    include snippets/shared-ssl.conf;
    ssl_certificate /cert-a.pem;
    ssl_certificate_key /key-a.pem;
}
server {
    listen 443 ssl;
    server_name b.example.com;
    include snippets/shared-ssl.conf;
    ssl_certificate /cert-b.pem;
    ssl_certificate_key /key-b.pem;
}`, parser.WithSkipValidDirectivesErr())

	conf, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	content := createMaintenanceConfig(conf, sitesAvailableDir)

	// The shared include must appear in both generated server blocks, proving
	// the visited cache does not leak across server-block expansions.
	if strings.Count(content, "ssl_protocols TLSv1.3;") != 2 {
		t.Fatalf("maintenance config = %q, want shared TLS directive expanded once per server block", content)
	}
	if !strings.Contains(content, "server_name a.example.com;") || !strings.Contains(content, "server_name b.example.com;") {
		t.Fatalf("maintenance config = %q, want both server_name directives preserved", content)
	}
}
