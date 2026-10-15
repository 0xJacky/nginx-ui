package ingress

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files in testdata")

// setupRender pins the values the renderer reads from the environment.
func setupRender(t *testing.T, http2Directive bool) {
	t.Helper()
	originalPort := settings.CertSettings.HTTPChallengePort
	originalAllowed := settings.IngressSettings.AllowedBackends
	originalHTTP2 := supportsHTTP2Directive
	settings.CertSettings.HTTPChallengePort = "9180"
	settings.IngressSettings.AllowedBackends = []string{"127.0.0.1/32", "::1/128"}
	supportsHTTP2Directive = func() bool { return http2Directive }
	t.Cleanup(func() {
		settings.CertSettings.HTTPChallengePort = originalPort
		settings.IngressSettings.AllowedBackends = originalAllowed
		supportsHTTP2Directive = originalHTTP2
	})
}

// goldenText joins the site and the upstream files the way the golden files
// store them.
func goldenText(r *Rendered) string {
	var b strings.Builder
	b.WriteString("# ---- site " + r.SiteName + "\n")
	b.WriteString(r.SiteContent)
	for _, name := range sortedKeys(r.Upstreams) {
		b.WriteString("# ---- upstream " + name + "\n")
		b.WriteString(r.Upstreams[name])
	}
	return b.String()
}

func TestRenderGolden(t *testing.T) {
	tests := []struct {
		name  string
		ing   *model.Ingress
		cert  CertState
		http2 bool
	}{
		{
			name: "basic_http",
			ing:  &model.Ingress{Name: "basic", Spec: basicSpec("app.example.com")},
		},
		{
			name: "multi_host_multi_route",
			ing: &model.Ingress{Name: "shop", Spec: model.IngressSpec{
				Hosts: []string{"shop.example.com", "www.shop.example.com"},
				Backends: []model.IngressBackend{
					{Name: "web", LB: model.IngressLBLeastConn, Keepalive: 16, Servers: []model.IngressServer{
						{Address: "127.0.0.1:24001", Weight: 3, MaxFails: 2, FailTimeout: "10s"},
						{Address: "127.0.0.1:24002", Down: true},
						{Address: "127.0.0.1:24003", Backup: true},
					}},
					{Name: "api", LB: model.IngressLBIPHash, Servers: []model.IngressServer{{Address: "[::1]:24010"}}},
				},
				Routes: []model.IngressRoute{
					{PathPrefix: "/", Backend: "web"},
					{PathPrefix: "/ws", Backend: "web", WebSocket: true, ReadTimeout: "300s", SendTimeout: "300s"},
					{PathPrefix: "/api/", Backend: "api", ClientMaxBodySize: "64m"},
				},
				TLS: model.IngressTLS{Mode: model.IngressTLSModeNone},
			}},
		},
		{
			name: "cidrs_rate_limit_headers",
			ing: &model.Ingress{Name: "guarded", Spec: model.IngressSpec{
				Hosts:    []string{"guarded.example.com"},
				Backends: basicSpec("x.example.com").Backends,
				Routes: []model.IngressRoute{
					{PathPrefix: "/", Backend: "app"},
					{PathPrefix: "/admin", Backend: "app", AllowCIDRs: []string{"10.0.0.0/8", "2001:db8::1"}},
				},
				TLS: model.IngressTLS{Mode: model.IngressTLSModeNone},
				Policy: model.IngressPolicy{
					AllowCIDRs:        []string{"203.0.113.0/24"},
					RateLimit:         &model.IngressRateLimit{RPS: 20, Burst: 40},
					ClientMaxBodySize: "16m",
					ExtraHeaders:      []model.IngressHeaderSet{{Name: "X-Request-Source", Value: "ingress controller"}},
				},
			}},
		},
		{
			name: "acme_pending",
			ing: &model.Ingress{Name: "pending", Spec: model.IngressSpec{
				Hosts:    []string{"pending.example.com"},
				Backends: basicSpec("x.example.com").Backends,
				Routes:   []model.IngressRoute{{PathPrefix: "/", Backend: "app", WebSocket: true}},
				TLS:      model.IngressTLS{Mode: model.IngressTLSModeACME, RedirectHTTPS: true, HSTS: true},
			}},
			cert: CertState{Reason: "Issuing"},
		},
		{
			name: "acme_ready_redirect_hsts",
			ing: &model.Ingress{Name: "secure", Spec: model.IngressSpec{
				Hosts:    []string{"secure.example.com"},
				Backends: basicSpec("x.example.com").Backends,
				Routes:   []model.IngressRoute{{PathPrefix: "/", Backend: "app", WebSocket: true}},
				TLS:      model.IngressTLS{Mode: model.IngressTLSModeACME, RedirectHTTPS: true, HSTS: true},
				Policy:   model.IngressPolicy{RateLimit: &model.IngressRateLimit{RPS: 5}},
			}},
			cert:  readyCert(),
			http2: true,
		},
		{
			name: "cert_ready_legacy_http2",
			ing: &model.Ingress{Name: "legacy", Spec: model.IngressSpec{
				Hosts:    []string{"*.legacy.example.com"},
				Backends: basicSpec("x.example.com").Backends,
				Routes:   []model.IngressRoute{{PathPrefix: "/", Backend: "app"}},
				TLS:      model.IngressTLS{Mode: model.IngressTLSModeCert, CertID: 3},
			}},
			cert: readyCert(),
		},
		{
			name: "idn_hosts",
			ing: &model.Ingress{Name: "idn", Spec: model.IngressSpec{
				Hosts:    []string{"例子。测试", "Bücher.Example.com", "ab--cd.example.com"},
				Backends: basicSpec("x.example.com").Backends,
				Routes:   basicSpec("x.example.com").Routes,
				TLS:      model.IngressTLS{Mode: model.IngressTLSModeNone},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupRender(t, tt.http2)
			rendered, err := Render(tt.ing, RenderOptions{Cert: tt.cert})
			require.NoError(t, err)

			again, err := Render(tt.ing, RenderOptions{Cert: tt.cert})
			require.NoError(t, err)
			assert.Equal(t, rendered, again, "rendering must be deterministic")
			assert.Equal(t, hashRendered(rendered), rendered.Hash)
			assert.Len(t, rendered.Hash, 64)

			golden := filepath.Join("testdata", "render", tt.name+".conf")
			got := goldenText(rendered)
			if *updateGolden {
				require.NoError(t, os.MkdirAll(filepath.Dir(golden), 0o755))
				require.NoError(t, os.WriteFile(golden, []byte(got), 0o644))
			}
			want, err := os.ReadFile(golden)
			require.NoError(t, err, "run go test -update to create %s", golden)
			assert.Equal(t, string(want), got)
		})
	}
}

func TestRenderHashChangesWithUpstreams(t *testing.T) {
	setupRender(t, true)
	ing := &model.Ingress{Name: "hash", Spec: basicSpec("hash.example.com")}
	first, err := Render(ing, RenderOptions{})
	require.NoError(t, err)

	ing.Spec.Backends[0].Servers[0].Down = true
	second, err := Render(ing, RenderOptions{})
	require.NoError(t, err)

	assert.Equal(t, first.SiteContent, second.SiteContent)
	assert.NotEqual(t, first.Hash, second.Hash)
}

func TestRenderRejectsUnsafeCertificatePath(t *testing.T) {
	setupRender(t, true)
	ing := &model.Ingress{Name: "unsafe", Spec: basicSpec("unsafe.example.com")}
	ing.Spec.TLS = model.IngressTLS{Mode: model.IngressTLSModeCert, CertID: 1}
	cert := readyCert()
	cert.CertPath = "/etc/nginx/ssl/a b;/fullchain.cer"
	_, err := Render(ing, RenderOptions{Cert: cert})
	requireCode(t, err, ErrCertNotReady)
}

func TestRenderNoneModeIgnoresCertificate(t *testing.T) {
	setupRender(t, true)
	ing := &model.Ingress{Name: "plain", Spec: basicSpec("plain.example.com")}
	rendered, err := Render(ing, RenderOptions{Cert: readyCert()})
	require.NoError(t, err)
	assert.NotContains(t, rendered.SiteContent, "443")
	assert.NotContains(t, rendered.SiteContent, "acme-challenge")
}

func settingsBackends(t *testing.T, allowed []string) {
	t.Helper()
	original := settings.IngressSettings.AllowedBackends
	settings.IngressSettings.AllowedBackends = allowed
	t.Cleanup(func() { settings.IngressSettings.AllowedBackends = original })
}
