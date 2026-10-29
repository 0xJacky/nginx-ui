package middleware

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newProxyTestResponse(body string) *http.Response {
	response := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
	response.Header.Set("ETag", "stale-etag")
	return response
}

const proxyTestPrivateKey = "-----BEGIN PRIVATE KEY-----\\nMIGH\\n-----END PRIVATE KEY-----\\n"

var readOnlyProxyRedactionCases = []struct {
	name    string
	route   string
	body    string
	secrets []string
	keep    []string
}{
	{
		name:    "notifier list",
		route:   "/api/external_notifies",
		body:    `{"data":[{"id":1,"type":"telegram","config":{"bot_token":"notify-secret"}}],"pagination":{"total":1}}`,
		secrets: []string{"notify-secret"},
		keep:    []string{`"bot_token"`, `"telegram"`},
	},
	{
		name:    "notifier detail",
		route:   "/api/external_notifies/:id",
		body:    `{"id":1,"type":"telegram","config":{"bot_token":"notify-secret"}}`,
		secrets: []string{"notify-secret"},
		keep:    []string{`"bot_token"`},
	},
	{
		name:    "site navigation",
		route:   "/api/site_navigation",
		body:    `{"data":[{"id":1,"name":"example.com","health_check_config":{"path":"/health","headers":{"Authorization":"Bearer probe-secret"}}}]}`,
		secrets: []string{"probe-secret"},
		keep:    []string{`"Authorization"`, `"/health"`},
	},
	{
		name:    "site health check",
		route:   "/api/site_navigation/health_check/:id",
		body:    `{"id":1,"host":"example.com:443","health_check_config":{"headers":{"Authorization":"Bearer probe-secret"}}}`,
		secrets: []string{"probe-secret"},
		keep:    []string{`"Authorization"`, `"example.com:443"`},
	},
	{
		name:    "site health check request secrets",
		route:   "/api/site_navigation/health_check/:id",
		body:    `{"id":1,"host":"example.com:443","health_check_config":{"method":"POST","body":"{\"password\":\"body-secret\"}","target_url":"https://probe:url-secret@example.com/health?token=query-secret","click_url":"https://example.com/status?key=click-secret"}}`,
		secrets: []string{"body-secret", "url-secret", "query-secret", "click-secret"},
		keep:    []string{`"POST"`, "example.com/health", "example.com/status", `token=`, `key=`},
	},
	{
		name:    "config history password file",
		route:   "/api/config_histories",
		body:    `{"data":[{"id":1,"name":".htpasswd","filepath":"/etc/nginx/auth/.htpasswd","content":"admin:htpasswd-secret"},{"id":2,"name":"signing.asc","filepath":"/etc/nginx/signing.asc","content":"-----BEGIN PGP PRIVATE KEY BLOCK-----\\npgp-secret"},{"id":3,"name":"nginx.conf","filepath":"/etc/nginx/nginx.conf","content":"events {}"}]}`,
		secrets: []string{"htpasswd-secret", "pgp-secret"},
		keep:    []string{"events {}"},
	},
	{
		name:    "certificate detail",
		route:   "/api/certs/:id",
		body:    `{"id":1,"name":"example","ssl_certificate":"-----BEGIN CERTIFICATE-----","ssl_certificate_key":"` + proxyTestPrivateKey + `","ssl_certificate_key_path":"/etc/nginx/ssl/example.key"}`,
		secrets: []string{"PRIVATE KEY"},
		keep:    []string{`"ssl_certificate_key_path"`, "BEGIN CERTIFICATE"},
	},
	{
		name:    "config history",
		route:   "/api/config_histories",
		body:    `{"data":[{"id":1,"name":"example.key","filepath":"/etc/nginx/ssl/example.key","content":"` + proxyTestPrivateKey + `"},{"id":2,"name":"nginx.conf","filepath":"/etc/nginx/nginx.conf","content":"events {}"}]}`,
		secrets: []string{"PRIVATE KEY"},
		keep:    []string{"events {}"},
	},
	{
		name:    "DNS domain list",
		route:   "/api/dns/domains",
		body:    `{"data":[{"id":1,"domain":"example.com","dns_credential":{"name":"DNS","config":{"configuration":{"credentials":{"TOKEN":"dns-secret"},"additional":{"SESSION":"dns-session-secret"}}}}}]}`,
		secrets: []string{"dns-secret", "dns-session-secret"},
		keep:    []string{`"TOKEN"`, `"example.com"`},
	},
	{
		name:    "DNS domain detail",
		route:   "/api/dns/domains/:id",
		body:    `{"id":1,"domain":"example.com","dns_credential":{"name":"DNS","config":{"configuration":{"credentials":{"TOKEN":"dns-secret"}}}}}`,
		secrets: []string{"dns-secret"},
		keep:    []string{`"TOKEN"`},
	},
}

func TestConfigureProxyResponseRedactsStoredSecretsForReadOnlyTokens(t *testing.T) {
	for _, tt := range readOnlyProxyRedactionCases {
		t.Run(tt.name, func(t *testing.T) {
			proxy := &httputil.ReverseProxy{}
			configureProxyResponse(proxy, 42, tt.route, true, true, nil)
			response := newProxyTestResponse(tt.body)

			require.NoError(t, proxy.ModifyResponse(response))
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			for _, secret := range tt.secrets {
				assert.NotContains(t, string(body), secret)
			}
			for _, kept := range tt.keep {
				assert.Contains(t, string(body), kept)
			}
			assert.Contains(t, string(body), settings.RedactedSensitiveValue)
			assert.Equal(t, int64(len(body)), response.ContentLength)
			assert.Equal(t, fmt.Sprint(len(body)), response.Header.Get("Content-Length"))
			assert.Empty(t, response.Header.Get("ETag"))
		})
	}
}

func TestConfigureProxyResponseKeepsStoredSecretsForManagementCallers(t *testing.T) {
	for _, tt := range readOnlyProxyRedactionCases {
		for _, caller := range []struct {
			name           string
			isServiceToken bool
		}{
			{"api:write token", true},
			{"interactive administrator", false},
		} {
			t.Run(tt.name+"/"+caller.name, func(t *testing.T) {
				proxy := &httputil.ReverseProxy{}
				configureProxyResponse(proxy, 42, tt.route, caller.isServiceToken, false, nil)
				response := newProxyTestResponse(tt.body)

				require.NoError(t, proxy.ModifyResponse(response))
				body, err := io.ReadAll(response.Body)
				require.NoError(t, err)
				assert.JSONEq(t, tt.body, string(body))
			})
		}
	}
}

func TestConfigureProxyResponseRefusesPrivateKeyFilesForReadOnlyTokens(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{"key extension", `{"name":"example.key","filepath":"/etc/nginx/ssl/example.key","content":"not-pem"}`},
		{"PEM private key content", `{"name":"bundle.pem","filepath":"/etc/nginx/ssl/bundle.pem","content":"` + proxyTestPrivateKey + `"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			proxy := &httputil.ReverseProxy{}
			configureProxyResponse(proxy, 42, "/api/config", true, true, nil)
			response := newProxyTestResponse(tt.body)

			require.NoError(t, proxy.ModifyResponse(response))
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			assert.Equal(t, http.StatusForbidden, response.StatusCode)
			assert.NotContains(t, string(body), "PRIVATE KEY")
			assert.NotContains(t, string(body), "not-pem")
			var payload map[string]any
			require.NoError(t, json.Unmarshal(body, &payload))
			assert.Equal(t, "config", payload["scope"])
			assert.EqualValues(t, 40019, payload["code"])
			assert.Equal(t, int64(len(body)), response.ContentLength)
		})
	}

	proxy := &httputil.ReverseProxy{}
	configureProxyResponse(proxy, 42, "/api/config", true, true, nil)
	plain := `{"name":"nginx.conf","filepath":"/etc/nginx/nginx.conf","content":"events {}"}`
	response := newProxyTestResponse(plain)
	require.NoError(t, proxy.ModifyResponse(response))
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.JSONEq(t, plain, string(body))
}

func TestConfigureProxyDirectorRequestsIdentityEncodingForReadOnlyRedaction(t *testing.T) {
	for _, route := range []string{"/api/config", "/api/certs/:id", "/api/external_notifies"} {
		assert.True(t, proxyRequiresIdentityEncoding(route, true, true), route)
		assert.False(t, proxyRequiresIdentityEncoding(route, true, false), route)
	}
	assert.True(t, proxyRequiresIdentityEncoding("/api/acme_users", true, false))
	assert.False(t, proxyRequiresIdentityEncoding("/api/sites", true, true))
}

func TestConfigureProxyResponseFailsClosedOnInvalidReadOnlyPayload(t *testing.T) {
	proxy := &httputil.ReverseProxy{}
	configureProxyResponse(proxy, 42, "/api/certs/:id", true, true, nil)
	response := newProxyTestResponse("not-json")
	require.Error(t, proxy.ModifyResponse(response))
}
