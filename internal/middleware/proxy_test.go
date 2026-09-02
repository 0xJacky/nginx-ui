package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestConfigureProxyDirectorUsesTargetHost(t *testing.T) {
	target, err := url.Parse("https://node.example.com:8443")
	require.NoError(t, err)

	proxy := httputil.NewSingleHostReverseProxy(target)
	configureProxyDirector(proxy)

	request := httptest.NewRequest(http.MethodGet, "https://dashboard.example.com/api/self_check?x_node_id=42&check=true", nil)
	request.Header.Set("X-Node-ID", "42")
	proxy.Director(request)

	assert.Equal(t, "node.example.com:8443", request.Host)
	assert.Equal(t, "true", request.URL.Query().Get("check"))
	assert.Empty(t, request.URL.Query().Get("x_node_id"))
	assert.Empty(t, request.Header.Get("X-Node-ID"))
}

func TestConfigureProxyResponseStatusMapping(t *testing.T) {
	tests := []struct {
		name           string
		upstreamStatus int
		downstream     int
		logged         bool
	}{
		{"forbidden", http.StatusForbidden, http.StatusServiceUnavailable, true},
		{"teapot", http.StatusTeapot, http.StatusTeapot, true},
		{"too many requests", http.StatusTooManyRequests, http.StatusTooManyRequests, true},
		{"internal server error", http.StatusInternalServerError, http.StatusInternalServerError, false},
		{"bad gateway", http.StatusBadGateway, http.StatusBadGateway, false},
		{"not found", http.StatusNotFound, http.StatusNotFound, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.WarnLevel)
			proxy := &httputil.ReverseProxy{}
			configureProxyResponse(proxy, 42, "/api/self_check", zap.New(core).Sugar().Warnw)

			request := httptest.NewRequest(http.MethodGet, "https://node.example.com/api/self_check?token=secret", nil)
			response := &http.Response{
				StatusCode:    tt.upstreamStatus,
				Header:        make(http.Header),
				Body:          io.NopCloser(strings.NewReader("response-secret")),
				ContentLength: int64(len("response-secret")),
				Request:       request,
			}
			response.Header.Set(proxyUpstreamStatusHeader, "spoofed")
			response.Header.Set("Access-Control-Allow-Origin", "https://example.com")
			response.Header.Set("CF-Ray", "abc123-SJC")

			require.NoError(t, proxy.ModifyResponse(response))
			assert.Equal(t, tt.downstream, response.StatusCode)
			assert.Empty(t, response.Header.Get("Access-Control-Allow-Origin"))
			assert.Equal(t, "abc123-SJC", response.Header.Get("CF-Ray"))
			if tt.upstreamStatus == http.StatusForbidden {
				assert.Equal(t, "403", response.Header.Get(proxyUpstreamStatusHeader))
			} else {
				assert.Empty(t, response.Header.Get(proxyUpstreamStatusHeader))
			}
			if tt.logged {
				assert.Equal(t, 1, logs.Len())
			} else {
				assert.Zero(t, logs.Len())
			}

			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			assert.Equal(t, "response-secret", string(body))
		})
	}
}

func TestProxyResponseDiagnosticClassifications(t *testing.T) {
	tests := []struct {
		name              string
		status            int
		hostMatchesTarget bool
		edgeChallenge     bool
		want              string
	}{
		{"host mismatch", http.StatusForbidden, false, false, "host_mismatch"},
		{"edge challenge", http.StatusForbidden, true, true, "edge_challenge"},
		{"rate limited", http.StatusTooManyRequests, true, false, "rate_limited"},
		{"upstream rejected", http.StatusTeapot, true, false, "upstream_rejected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, proxyResponseClassification(tt.status, tt.hostMatchesTarget, tt.edgeChallenge))
		})
	}
}

func TestProxyResponseDiagnosticDoesNotLeakSensitiveData(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	proxy := &httputil.ReverseProxy{}
	configureProxyResponse(proxy, 42, "/api/nginx_log/search", zap.New(core).Sugar().Warnw)

	request := httptest.NewRequest(http.MethodPost,
		"https://node.example.com/api/nginx_log/search?token=query-secret&search=union+select",
		strings.NewReader("request-secret"))
	request.Header.Set("Authorization", "authorization-secret")
	request.Header.Set("Cookie", "cookie-secret")
	request.Header.Set("Signature", "signature-secret")
	request.Header.Set("Signature-Input", "signature-input-secret")
	request.Header.Set("Content-Digest", "digest-secret")
	request.Header.Set("X-Nginx-UI-Credential-ID", "credential-secret")
	request.Header.Set("X-Nginx-UI-Target-Instance", "target-instance-secret")
	responseHeader := make(http.Header)
	responseHeader.Set("Content-Type", "text/html")
	responseHeader.Set("Set-Cookie", "response-cookie-secret")
	responseHeader.Set("CF-Ray", "abc123-SJC")
	response := &http.Response{
		StatusCode:    http.StatusForbidden,
		Header:        responseHeader,
		Body:          io.NopCloser(strings.NewReader("response-secret")),
		ContentLength: int64(len("response-secret")),
		Request:       request,
	}

	require.NoError(t, proxy.ModifyResponse(response))
	require.Equal(t, 1, logs.Len())
	entry := logs.All()[0]
	logged := entry.Message + fmt.Sprint(entry.ContextMap())
	for _, secret := range []string{
		"query-secret",
		"union",
		"request-secret",
		"authorization-secret",
		"cookie-secret",
		"signature-secret",
		"signature-input-secret",
		"digest-secret",
		"credential-secret",
		"target-instance-secret",
		"response-cookie-secret",
		"response-secret",
	} {
		assert.NotContains(t, logged, secret)
	}
	assert.Contains(t, logged, "/api/nginx_log/search")
	assert.Contains(t, logged, "abc123-SJC")
}

func TestBoundedLogValue(t *testing.T) {
	assert.Equal(t, "valid�header", boundedLogValue("  valid\xffheader  "))
	assert.Equal(t, strings.Repeat("界", 128), boundedLogValue(strings.Repeat("界", 129)))
}
