package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"testing"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The controller asks the child to classify secret files only for a
// read-only caller and only on the routes that read configuration files, and
// a client can neither add the marker nor keep one it supplied.
// (GHSA-x49v-g3gc-3w78)
func TestSetProxyReadOnlyServiceTokenIsTheControllersDecision(t *testing.T) {
	for _, tt := range []struct {
		name     string
		route    string
		readOnly bool
		marked   bool
	}{
		{"read-only file read", "/api/config", true, true},
		{"read-only history", "/api/config_histories", true, true},
		{"management file read", "/api/config", false, false},
		{"read-only other route", "/api/sites", true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet,
				"https://node.example.com/api/config?path=nginx.conf&"+proxyReadOnlyServiceTokenQuery+"=false&"+proxyReadOnlyServiceTokenQuery+"=x", nil)
			setProxyReadOnlyServiceToken(request, tt.route, tt.readOnly)
			values := request.URL.Query()
			assert.Equal(t, "nginx.conf", values.Get("path"))
			if tt.marked {
				assert.Equal(t, []string{"true"}, values[proxyReadOnlyServiceTokenQuery])
			} else {
				assert.False(t, values.Has(proxyReadOnlyServiceTokenQuery))
			}
		})
	}
}

// A child withholds secret files from a relayed read-only caller and says so;
// the marker means nothing without node authentication, and a node request
// without it keeps full access, as cluster synchronization needs.
func TestWithholdSecretFilesHonorsRelayedReadOnlyCallers(t *testing.T) {
	marked := "/api/config?path=nginx.conf&" + proxyReadOnlyServiceTokenQuery + "=true"
	for _, tt := range []struct {
		name      string
		url       string
		node      bool
		readOnly  bool
		withholds bool
		confirms  bool
	}{
		{"relayed read-only caller", marked, true, false, true, true},
		{"node request without marker", "/api/config?path=nginx.conf", true, false, false, false},
		{"marker without node authentication", marked, false, false, false, false},
		{"local read-only service token", "/api/config?path=nginx.conf", false, true, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestGinContext(t, http.MethodGet, tt.url, nil)
			if tt.node {
				c.Set(nodeauth.GinPrincipalKey, &nodeauth.Principal{CredentialID: "legacy"})
			}
			if tt.readOnly {
				c.Set(internalmcp.ServiceTokenPrincipalKey, &internalmcp.ServiceTokenPrincipal{
					Scopes: []string{model.APITokenScopeRead},
				})
			}
			assert.Equal(t, tt.withholds, WithholdSecretFiles(c))
			if tt.confirms {
				assert.Equal(t, "?1", c.Writer.Header().Get(secretFilesWithheldHeader))
			} else {
				assert.Empty(t, c.Writer.Header().Get(secretFilesWithheldHeader))
			}
		})
	}
}

// A child that does not confirm predates the exchange, so nothing tells its
// secret files apart: the controller refuses every file read and redacts every
// history entry for read-only callers rather than relay a secret.
func TestConfigureProxyResponseWithholdsFilesFromUnconfirmedChild(t *testing.T) {
	t.Run("file read", func(t *testing.T) {
		proxy := &httputil.ReverseProxy{}
		configureProxyResponse(proxy, 42, "/api/config", true, true, nil)
		response := newProxyTestResponse(`{"name":"ticket.bin","filepath":"/etc/nginx/ssl/ticket.bin","content":"ticket-secret"}`)

		require.NoError(t, proxy.ModifyResponse(response))
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, response.StatusCode)
		assert.NotContains(t, string(body), "ticket-secret")
		var payload map[string]any
		require.NoError(t, json.Unmarshal(body, &payload))
		assert.Equal(t, "middleware", payload["scope"])
		assert.EqualValues(t, 40304, payload["code"])
	})

	t.Run("history", func(t *testing.T) {
		proxy := &httputil.ReverseProxy{}
		configureProxyResponse(proxy, 42, "/api/config_histories", true, true, nil)
		response := newProxyTestResponse(`{"data":[{"id":1,"name":"ticket.bin","filepath":"/etc/nginx/ssl/ticket.bin","content":"ticket-secret"},{"id":2,"name":"nginx.conf","filepath":"/etc/nginx/nginx.conf","content":"events {}"}],"pagination":{"total":2}}`)

		require.NoError(t, proxy.ModifyResponse(response))
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.NotContains(t, string(body), "ticket-secret")
		assert.NotContains(t, string(body), "events {}")
		assert.Contains(t, string(body), `"nginx.conf"`)
		assert.Equal(t, 2, strings.Count(string(body), settings.RedactedSensitiveValue))
	})

	t.Run("management callers", func(t *testing.T) {
		plain := `{"name":"ticket.bin","filepath":"/etc/nginx/ssl/ticket.bin","content":"ticket-secret"}`
		proxy := &httputil.ReverseProxy{}
		configureProxyResponse(proxy, 42, "/api/config", true, false, nil)
		response := newProxyTestResponse(plain)
		require.NoError(t, proxy.ModifyResponse(response))
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.JSONEq(t, plain, string(body))
	})
}

// The child's own refusal reaches a read-only caller as the 403 the local
// handler returns, and the confirmation itself is never relayed. Any other
// upstream 403 keeps the proxy's usual 503.
func TestConfigureProxyResponseRelaysConfirmedChildRefusal(t *testing.T) {
	refusal := `{"scope":"config","code":40019,"message":"secret files are hidden from read-only service tokens"}`
	for _, tt := range []struct {
		name      string
		readOnly  bool
		confirmed bool
		status    int
	}{
		{"confirmed refusal", true, true, http.StatusForbidden},
		{"unconfirmed 403", true, false, http.StatusServiceUnavailable},
		{"management caller", false, true, http.StatusServiceUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			proxy := &httputil.ReverseProxy{}
			configureProxyResponse(proxy, 42, "/api/config", true, tt.readOnly, nil)
			response := newProxyTestResponse(refusal)
			response.StatusCode = http.StatusForbidden
			if tt.confirmed {
				response.Header.Set(secretFilesWithheldHeader, "?1")
			}

			require.NoError(t, proxy.ModifyResponse(response))
			assert.Equal(t, tt.status, response.StatusCode)
			assert.Empty(t, response.Header.Get(secretFilesWithheldHeader))
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			assert.JSONEq(t, refusal, string(body))
		})
	}
}
