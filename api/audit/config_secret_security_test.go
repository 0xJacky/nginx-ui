package audit

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	syntheticFileSecret = "synthetic-file-secret"
	syntheticPEMKey     = "-----BEGIN PRIVATE KEY-----\\nc3ludGhldGlj\\n-----END PRIVATE KEY-----\\n"
)

// useTempNginxConf points the configuration root at a temporary directory
// whose site configuration names ssl/site.der as a certificate key, so the
// file is secret only through that reference.
func useTempNginxConf(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	original := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = root
	t.Cleanup(func() { settings.NginxSettings.ConfigDir = original })
	for name, content := range map[string]string{
		"ssl/site.der":       "\x30\x82binary",
		"conf.d/site.conf":   "server {\n    ssl_certificate_key ssl/site.der;\n}\n",
		"conf.d/public.conf": "server { listen 80; }\n",
	} {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
}

// respondWith returns a handler that consumes the request body and answers
// with body, optionally as if a remote node had answered through the proxy.
func respondWith(body string, proxied bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if proxied {
			c.Set("ProxyNodeID", uint64(7))
		}
		var payload map[string]any
		_ = c.ShouldBindJSON(&payload)
		c.Data(http.StatusOK, "application/json", []byte(body))
	}
}

type configAuditCase struct {
	name     string
	method   string
	route    string
	target   string
	reqBody  string
	respBody string
	proxied  bool
	// redacted is true when the request reads or writes a secret file.
	redacted bool
}

func TestAuditLogRedactsSecretConfigurationFiles(t *testing.T) {
	useTempNginxConf(t)

	cases := []configAuditCase{
		{
			name: "read password file", method: http.MethodGet, route: "/api/config", target: "/api/config?path=auth/users.htpasswd",
			respBody: `{"name":"users.htpasswd","content":"admin:` + syntheticFileSecret + `","filepath":"/etc/nginx/auth/users.htpasswd"}`, redacted: true,
		},
		{
			name: "read key referenced by a directive", method: http.MethodGet, route: "/api/config", target: "/api/config?path=ssl/site.der",
			respBody: `{"name":"site.der","content":"` + syntheticFileSecret + `","filepath":"ssl/site.der"}`, redacted: true,
		},
		{
			name: "proxied read of a private key", method: http.MethodGet, route: "/api/config", target: "/api/config?path=ssl/remote.pem",
			respBody: `{"name":"remote.pem","content":"` + syntheticPEMKey + syntheticFileSecret + `","filepath":"/etc/nginx/ssl/remote.pem"}`, proxied: true, redacted: true,
		},
		{
			name: "proxied read of a key file", method: http.MethodGet, route: "/api/config", target: "/api/config?path=ssl/remote.key",
			respBody: `{"name":"remote.key","content":"` + syntheticFileSecret + `"}`, proxied: true, redacted: true,
		},
		{
			name: "edit key referenced by a directive", method: http.MethodPost, route: "/api/config", target: "/api/config",
			reqBody: `{"path":"ssl/site.der","content":"` + syntheticFileSecret + `"}`, respBody: `{"content":"` + syntheticFileSecret + `"}`, redacted: true,
		},
		{
			name: "create key file", method: http.MethodPost, route: "/api/configs", target: "/api/configs",
			reqBody: `{"name":"new.key","base_dir":"ssl","content":"` + syntheticFileSecret + `"}`, respBody: `{"content":"` + syntheticFileSecret + `"}`, redacted: true,
		},
		{
			name: "history of a key file", method: http.MethodGet, route: "/api/config_histories", target: "/api/config_histories?filepath=/etc/nginx/ssl/site.key",
			respBody: `{"data":[{"filepath":"/etc/nginx/ssl/site.key","content":"` + syntheticFileSecret + `"}]}`, redacted: true,
		},
		{
			name: "MCP read of a key file", method: http.MethodPost, route: "/mcp", target: "/mcp",
			reqBody:  `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nginx_config_get","arguments":{"relative_path":"ssl/site.key"}}}`,
			respBody: `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"` + syntheticFileSecret + `"}]}}`, redacted: true,
		},
		{
			name: "MCP edit of a key referenced by a directive", method: http.MethodPost, route: "/mcp", target: "/mcp",
			reqBody:  `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nginx_config_modify","arguments":{"relative_path":"ssl/site.der","content":"` + syntheticFileSecret + `"}}}`,
			respBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, redacted: true,
		},
		{
			name: "private key in any body", method: http.MethodPost, route: "/api/unlisted", target: "/api/unlisted",
			reqBody: `{"note":"` + syntheticPEMKey + syntheticFileSecret + `"}`, respBody: `{"echo":"` + syntheticPEMKey + syntheticFileSecret + `"}`, redacted: true,
		},
		{
			name: "read ordinary configuration", method: http.MethodGet, route: "/api/config", target: "/api/config?path=conf.d/public.conf",
			respBody: `{"name":"public.conf","content":"server { listen 80; } ` + syntheticFileSecret + `","filepath":"conf.d/public.conf"}`,
		},
		{
			name: "edit ordinary configuration", method: http.MethodPost, route: "/api/config", target: "/api/config",
			reqBody: `{"path":"conf.d/public.conf","content":"server { listen 80; } ` + syntheticFileSecret + `"}`, respBody: `{"message":"ok"}`,
		},
		{
			name: "other MCP tool", method: http.MethodPost, route: "/mcp", target: "/mcp",
			reqBody:  `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nginx_site_list","arguments":{"filter":"` + syntheticFileSecret + `"}}}`,
			respBody: `{"jsonrpc":"2.0","id":1,"result":{}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := newAuditedServer(t, func(engine *gin.Engine) {
				engine.Handle(tc.method, tc.route, respondWith(tc.respBody, tc.proxied))
			})
			server.send(t, tc.method, tc.target, tc.reqBody)
			entry := server.waitForLogs(t, 1)[0]
			if tc.redacted {
				assert.NotContains(t, entry["req_body"], syntheticFileSecret)
				assert.NotContains(t, entry["resp_body"], syntheticFileSecret)
				assert.NotContains(t, entry["session_logs"], syntheticFileSecret)
				return
			}
			assert.Equal(t, tc.respBody, entry["resp_body"])
			if tc.reqBody != "" {
				assert.Equal(t, tc.reqBody, entry["req_body"])
			}
		})
	}
}

func TestAuditLogRedactsDNSDomainCredentials(t *testing.T) {
	for _, route := range []string{"/api/dns/domains", "/api/dns/domains/:id"} {
		t.Run(route, func(t *testing.T) {
			body := `{"id":1,"domain":"example.com","dns_credential":{"config":{"configuration":{"credentials":{"X":"` + syntheticFileSecret + `"}}}}}`
			server := newAuditedServer(t, func(engine *gin.Engine) {
				engine.GET(route, respondWith(body, false))
			})
			server.send(t, http.MethodGet, strings.Replace(route, ":id", "1", 1), "")
			entry := server.waitForLogs(t, 1)[0]
			assert.NotContains(t, entry["resp_body"], syntheticFileSecret)
		})
	}
}
