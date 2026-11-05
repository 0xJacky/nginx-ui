package audit

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	syntheticSecret     = "synthetic-credential-value"
	syntheticSQLSession = `[{"sql":"INSERT INTO t VALUES ('synthetic-credential-value')"}]`
)

type auditCase struct {
	method   string
	route    string
	target   string
	reqBody  string
	respBody string
	// handler, when set, replaces the default handler (used to simulate the
	// proxy middleware answering before the real handler runs).
	handler     gin.HandlerFunc
	reqHeaders  map[string]string
	respHeaders map[string]string
	sessionLogs string
}

// runAudit serves one request through a real gin route so c.FullPath() is
// populated, then sanitizes a logMap shaped like the one cosy builds.
func runAudit(t *testing.T, tc auditCase) map[string]string {
	t.Helper()
	gin.SetMode(gin.TestMode)

	logMap := map[string]string{
		"req_body":     tc.reqBody,
		"resp_body":    tc.respBody,
		"session_logs": tc.sessionLogs,
	}
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Next()
		sanitizeAuditLog(c, logMap)
	})
	handler := tc.handler
	if handler == nil {
		handler = func(c *gin.Context) {
			for name, value := range tc.respHeaders {
				c.Header(name, value)
			}
			c.String(http.StatusOK, tc.respBody)
		}
	}
	engine.Handle(tc.method, tc.route, handler)

	request := httptest.NewRequest(tc.method, tc.target, bytes.NewBufferString(tc.reqBody))
	request.Header.Set("Content-Type", "application/json")
	for name, value := range tc.reqHeaders {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	return logMap
}

func TestSanitizeAuditLogRedactsCredentialBearingRequests(t *testing.T) {
	cases := []auditCase{
		{method: http.MethodPost, route: "/api/user/password", target: "/api/user/password", reqBody: `{"old_password":"` + syntheticSecret + `","new_password":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/users", target: "/api/users", reqBody: `{"name":"u","password":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/users/:id", target: "/api/users/2", reqBody: `{"name":"u","password":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/setup/install", target: "/api/setup/install", reqBody: `{"encrypted_params":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/settings", target: "/api/settings", reqBody: `{"oidc":{"client_secret":"` + syntheticSecret + `"}}`},
		{method: http.MethodPost, route: "/api/settings/:section", target: "/api/settings/openai", reqBody: `{"tok":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/settings/nginx/private-key", target: "/api/settings/nginx/private-key", reqBody: `{"pk":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/dns_credentials", target: "/api/dns_credentials", reqBody: `{"configuration":{"credentials":{"X":"` + syntheticSecret + `"}}}`},
		{method: http.MethodPost, route: "/api/dns_credentials/:id", target: "/api/dns_credentials/1", reqBody: `{"configuration":{"credentials":{"X":"` + syntheticSecret + `"}}}`},
		{method: http.MethodPost, route: "/api/acme_users", target: "/api/acme_users", reqBody: `{"eab":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/acme_users/:id", target: "/api/acme_users/1", reqBody: `{"eab":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/certs", target: "/api/certs", reqBody: `{"pem":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/certs/:id", target: "/api/certs/1", reqBody: `{"pem":"` + syntheticSecret + `"}`},
		{method: http.MethodPut, route: "/api/cert_sync", target: "/api/cert_sync", reqBody: `{"pem":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/certs/:id/download", target: "/api/certs/1/download", reqBody: `{"pfx":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/external_notifies", target: "/api/external_notifies", reqBody: `{"config":{"x":"` + syntheticSecret + `"}}`},
		{method: http.MethodPost, route: "/api/external_notifies/:id", target: "/api/external_notifies/1", reqBody: `{"config":{"x":"` + syntheticSecret + `"}}`},
		{method: http.MethodPost, route: "/api/external_notifies/test", target: "/api/external_notifies/test", reqBody: `{"config":{"x":"` + syntheticSecret + `"}}`},
		{method: http.MethodPost, route: "/api/auto_backup", target: "/api/auto_backup", reqBody: `{"s3":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/auto_backup/:id", target: "/api/auto_backup/1", reqBody: `{"s3":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/auto_backup/test_s3", target: "/api/auto_backup/test_s3", reqBody: `{"s3":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/nodes", target: "/api/nodes", reqBody: `{"t":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/llm/models", target: "/api/llm/models", reqBody: `{"t":"` + syntheticSecret + `"}`},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.route, func(t *testing.T) {
			tc.sessionLogs = syntheticSQLSession
			logMap := runAudit(t, tc)
			assert.Equal(t, "[sensitive request redacted]", logMap["req_body"])
			assert.Equal(t, "[sensitive session logs redacted]", logMap["session_logs"])
		})
	}
}

func TestSanitizeAuditLogRedactsCredentialBearingResponses(t *testing.T) {
	cases := []auditCase{
		{method: http.MethodPost, route: "/api/token/short", target: "/api/token/short", respBody: `{"st":"` + syntheticSecret + `"}`},
		{method: http.MethodGet, route: "/api/settings/protected", target: "/api/settings/protected?path=oidc.client_secret", respBody: `{"value":"` + syntheticSecret + `"}`},
		{method: http.MethodGet, route: "/api/certs/:id", target: "/api/certs/1", respBody: `{"pem":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/self_signed_cert", target: "/api/self_signed_cert", respBody: `{"pem":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/certs/:id/download", target: "/api/certs/1/download", respBody: syntheticSecret},
		{method: http.MethodGet, route: "/api/dns_credentials/:id", target: "/api/dns_credentials/1", respBody: `{"configuration":{"credentials":{"X":"` + syntheticSecret + `"}}}`},
		{method: http.MethodGet, route: "/api/dns_credentials", target: "/api/dns_credentials", respBody: `{"data":[{"configuration":{"credentials":{"X":"` + syntheticSecret + `"}}}]}`},
		{method: http.MethodGet, route: "/api/acme_users/:id", target: "/api/acme_users/1", respBody: `{"eab":"` + syntheticSecret + `"}`},
		{method: http.MethodGet, route: "/api/external_notifies/:id", target: "/api/external_notifies/1", respBody: `{"config":{"x":"` + syntheticSecret + `"}}`},
		{method: http.MethodGet, route: "/api/auto_backup/:id", target: "/api/auto_backup/1", respBody: `{"s3":"` + syntheticSecret + `"}`},
		{method: http.MethodGet, route: "/api/nodes/:id/secret", target: "/api/nodes/1/secret", respBody: `{"value":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/host/setup/keypair", target: "/api/host/setup/keypair", respBody: `{"pk":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/service_tokens", target: "/api/service_tokens", respBody: `{"t":"` + syntheticSecret + `"}`},
		{method: http.MethodPost, route: "/api/mcp/tokens/:id/rotate", target: "/api/mcp/tokens/1/rotate", respBody: `{"t":"` + syntheticSecret + `"}`},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.route, func(t *testing.T) {
			logMap := runAudit(t, tc)
			assert.Equal(t, "[sensitive response redacted]", logMap["resp_body"])
		})
	}
}

// TestSanitizeAuditLogRedactsProxiedSensitiveRoutes covers requests for a
// remote node: the proxy middleware answers before the local handler runs, so
// redaction must not depend on the handler marking the request.
func TestSanitizeAuditLogRedactsProxiedSensitiveRoutes(t *testing.T) {
	logMap := runAudit(t, auditCase{
		method:   http.MethodGet,
		route:    "/api/dns_credentials/:id",
		target:   "/api/dns_credentials/1",
		respBody: `{"configuration":{"credentials":{"X":"` + syntheticSecret + `"}}}`,
		handler: func(c *gin.Context) {
			c.String(http.StatusOK, `{"configuration":{"credentials":{"X":"`+syntheticSecret+`"}}}`)
			c.Abort()
		},
	})
	assert.Equal(t, "[sensitive response redacted]", logMap["resp_body"])
}

func TestSanitizeAuditLogRedactsBackupAndInstallSecrets(t *testing.T) {
	logMap := runAudit(t, auditCase{
		method:      http.MethodGet,
		route:       "/api/backup",
		target:      "/api/backup",
		respBody:    syntheticSecret,
		respHeaders: map[string]string{"X-Backup-Security": syntheticSecret},
	})
	assert.NotContains(t, logMap["resp_header"], syntheticSecret)
	assert.Contains(t, logMap["resp_header"], "X-Backup-Security")
	assert.Equal(t, "[sensitive response redacted]", logMap["resp_body"])

	logMap = runAudit(t, auditCase{
		method:     http.MethodGet,
		route:      "/api/setup/self_check",
		target:     "/api/setup/self_check?install_secret=" + syntheticSecret + "&lang=en",
		reqHeaders: map[string]string{"X-Install-Secret": syntheticSecret},
	})
	assert.NotContains(t, logMap["req_header"], syntheticSecret)
	assert.NotContains(t, logMap["req_url"], syntheticSecret)
	assert.Contains(t, logMap["req_url"], "lang=en")
}

func TestSanitizeAuditLogKeepsExistingHeaderAndQueryRedaction(t *testing.T) {
	logMap := runAudit(t, auditCase{
		method: http.MethodGet,
		route:  "/api/nginx/status",
		target: "/api/nginx/status?token=" + syntheticSecret + "&node_secret=" + syntheticSecret + "&mfa_pre_auth=" + syntheticSecret + "&page=2",
		reqHeaders: map[string]string{
			"Authorization":       "Bearer " + syntheticSecret,
			"X-Node-Secret":       syntheticSecret,
			"Cookie":              "session=" + syntheticSecret,
			"X-Secure-Session-ID": syntheticSecret,
			"X-Current-Password":  syntheticSecret,
			"X-Api-Key":           syntheticSecret,
			"X-Request-Origin":    "kept-header-value",
		},
		respHeaders: map[string]string{"Set-Cookie": "session=" + syntheticSecret},
	})
	assert.NotContains(t, logMap["req_header"], syntheticSecret)
	assert.Contains(t, logMap["req_header"], "kept-header-value")
	assert.NotContains(t, logMap["resp_header"], syntheticSecret)
	assert.NotContains(t, logMap["req_url"], syntheticSecret)
	assert.Contains(t, logMap["req_url"], "page=2")
}

// TestSanitizeAuditLogKeepsOrdinaryBodies makes sure routine configuration
// edits remain fully auditable.
func TestSanitizeAuditLogKeepsOrdinaryBodies(t *testing.T) {
	siteBody := `{"content":"server {\n    listen 80;\n    proxy_set_header \"Host\" $host;\n}\n","tokenized":null,"max_tokens":3,"public_key":"ssh-ed25519 AAAA","key_type":"P256","site_key":"example"}`
	respBody := `{"name":"example.com","enabled":true}`
	logMap := runAudit(t, auditCase{
		method:      http.MethodPost,
		route:       "/api/sites/:name",
		target:      "/api/sites/example.com",
		reqBody:     siteBody,
		respBody:    respBody,
		sessionLogs: `[{"sql":"SELECT 1"}]`,
	})
	assert.Equal(t, siteBody, logMap["req_body"])
	assert.Equal(t, respBody, logMap["resp_body"])
	assert.Equal(t, `[{"sql":"SELECT 1"}]`, logMap["session_logs"])
}

// TestSanitizeAuditLogRedactsCredentialFieldsInKeptBodies is the fallback for
// handlers that are not in the route table.
func TestSanitizeAuditLogRedactsCredentialFieldsInKeptBodies(t *testing.T) {
	body := `{"name":"n","password":"` + syntheticSecret + `","config":{"bot_token":"` + syntheticSecret + `","ALICLOUD_SECRET_KEY":"` + syntheticSecret + `","apiKey":"` + syntheticSecret + `","ssl_certificate_key":"` + syntheticSecret + `","ssl_certificate_key_path":"/etc/nginx/ssl/k.pem"},"empty_token":""}`
	logMap := runAudit(t, auditCase{
		method:   http.MethodPost,
		route:    "/api/unlisted",
		target:   "/api/unlisted",
		reqBody:  body,
		respBody: `{"access_token":"` + syntheticSecret,
	})
	assert.NotContains(t, logMap["req_body"], syntheticSecret)
	assert.Contains(t, logMap["req_body"], `"name":"n"`)
	assert.Contains(t, logMap["req_body"], `"ssl_certificate_key_path":"/etc/nginx/ssl/k.pem"`)
	assert.Contains(t, logMap["req_body"], `"empty_token":""`)
	assert.Contains(t, logMap["req_body"], `"password":"[REDACTED]"`)
	// A body cut off by the audit size limit must not leak a partial value.
	assert.NotContains(t, logMap["resp_body"], syntheticSecret)
}
