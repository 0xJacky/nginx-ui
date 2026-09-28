package template

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildQuickConfigReverseProxyToUpstream(t *testing.T) {
	req := QuickConfigRequest{
		Type:            QuickConfigTypeReverseProxy,
		Domains:         []string{"a.example.com"},
		Scheme:          "http",
		Upstream:        "backend_pool",
		EnableWebSocket: true,
	}
	req.fillDefaults()
	require.NoError(t, req.validate())

	cfg, err := buildQuickConfig(&req)
	require.NoError(t, err)

	location := findLocation(cfg.Servers[0], "/")
	assert.Contains(t, location.Content, "proxy_pass http://backend_pool/;")
	assert.NotContains(t, location.Content, "127.0.0.1")

	content, err := cfg.BuildConfig()
	require.NoError(t, err)
	_, err = nginx.ParseNgxConfigByContent(content)
	require.NoError(t, err)
}

func TestBuildQuickConfigReverseProxyToUpstreamOverHTTPS(t *testing.T) {
	req := QuickConfigRequest{
		Type:     QuickConfigTypeReverseProxy,
		Domains:  []string{"a.example.com"},
		Scheme:   "https",
		Upstream: "tls_pool",
	}
	req.fillDefaults()
	require.NoError(t, req.validate())

	cfg, err := buildQuickConfig(&req)
	require.NoError(t, err)
	assert.Contains(t, findLocation(cfg.Servers[0], "/").Content, "proxy_pass https://tls_pool/;")
}

func TestQuickConfigRejectsInvalidUpstreamName(t *testing.T) {
	for _, name := range []string{"bad name", "pool;", "a{b}", "pool.example.com", "1pool"} {
		req := QuickConfigRequest{
			Type:     QuickConfigTypeReverseProxy,
			Domains:  []string{"a.example.com"},
			Upstream: name,
		}
		req.fillDefaults()
		assert.Error(t, req.validate(), name)
	}
}

func TestAnalyzeQuickConfigDetectsUpstreamTarget(t *testing.T) {
	original := isKnownUpstream
	isKnownUpstream = func(name string) bool { return name == "backend_pool" }
	t.Cleanup(func() { isKnownUpstream = original })

	cfg, err := nginx.ParseNgxConfigByContent(`server {
    listen 80;
    server_name a.example.com;
    location / {
        proxy_pass http://backend_pool/;
    }
}`)
	require.NoError(t, err)

	req := analyzeNgxConfig(cfg)
	assert.Equal(t, QuickConfigTypeReverseProxy, req.Type)
	assert.Equal(t, "backend_pool", req.Upstream)

	// A plain host without a port that is not an upstream stays a host.
	cfg, err = nginx.ParseNgxConfigByContent(`server {
    listen 80;
    server_name b.example.com;
    location / {
        proxy_pass http://localhost/;
    }
}`)
	require.NoError(t, err)
	req = analyzeNgxConfig(cfg)
	assert.Empty(t, req.Upstream)
	assert.Equal(t, "localhost", req.Host)
}
