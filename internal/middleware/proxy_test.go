package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
