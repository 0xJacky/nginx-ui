package site

import (
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testChallengeLocation = &nginx.NgxLocation{
	Path:    "~ /.well-known/acme-challenge",
	Content: "proxy_set_header Host $host;\nproxy_pass http://127.0.0.1:9180;\n",
}

const (
	testCertPath = "/etc/nginx/ssl/example.com_P256/fullchain.cer"
	testKeyPath  = "/etc/nginx/ssl/example.com_P256/private.key"
)

func mustParseNgxConfig(t *testing.T, content string) *nginx.NgxConfig {
	t.Helper()
	cfg, err := nginx.ParseNgxConfigByContent(content)
	require.NoError(t, err)
	return cfg
}

// rebuild renders cfg and parses it again, proving the output is valid for the
// same parser the site editor uses.
func rebuild(t *testing.T, cfg *nginx.NgxConfig) (string, *nginx.NgxConfig) {
	t.Helper()
	content, err := cfg.BuildConfig()
	require.NoError(t, err)
	parsed, err := nginx.ParseNgxConfigByContent(content)
	require.NoError(t, err, content)
	return content, parsed
}

func tlsServers(cfg *nginx.NgxConfig) []*nginx.NgxServer {
	var servers []*nginx.NgxServer
	for _, server := range cfg.Servers {
		if tls, _ := serverListenKinds(server); tls {
			servers = append(servers, server)
		}
	}
	return servers
}

func plainServers(cfg *nginx.NgxConfig) []*nginx.NgxServer {
	var servers []*nginx.NgxServer
	for _, server := range cfg.Servers {
		if tls, plain := serverListenKinds(server); !tls && plain {
			servers = append(servers, server)
		}
	}
	return servers
}

func directiveValues(server *nginx.NgxServer, name string) []string {
	var values []string
	for _, directive := range server.Directives {
		if directive.Directive == name {
			values = append(values, trimParams(directive.Params))
		}
	}
	return values
}

func challengeCount(server *nginx.NgxServer) int {
	count := 0
	for _, location := range server.Locations {
		if isChallengeLocation(location) {
			count++
		}
	}
	return count
}

func rootLocationContent(server *nginx.NgxServer) (string, bool) {
	for _, location := range server.Locations {
		if isRootLocation(location) {
			return location.Content, true
		}
	}
	return "", false
}

// serverText renders a single server so assertions can look at its text.
func serverText(t *testing.T, server *nginx.NgxServer) string {
	t.Helper()
	content, err := (&nginx.NgxConfig{Servers: []*nginx.NgxServer{server}}).BuildConfig()
	require.NoError(t, err)
	return content
}

func assertNoHTTPSRedirect(t *testing.T, server *nginx.NgxServer) {
	t.Helper()
	text := serverText(t, server)
	assert.NotContains(t, text, "https://$host", "unexpected HTTPS redirect:\n%s", text)
	assert.NotContains(t, text, "https://$server_name", "unexpected HTTPS redirect:\n%s", text)
}

func assertCanonicalRedirect(t *testing.T, server *nginx.NgxServer) {
	t.Helper()
	content, ok := rootLocationContent(server)
	require.True(t, ok, "port-80 server has no location /")
	assert.True(t, contentRedirectsOnlyToHTTPS(content), "location / is not a redirect: %q", content)
	assert.Empty(t, directiveValues(server, "return"), "redirect must not be a server-level return")
	assert.Equal(t, 1, challengeCount(server))
}

const quickSetupRedirectConfig = `
server {
    listen 80;
    listen [::]:80;
    server_name example.com;
    location / {
        return 301 https://$host$request_uri;
    }
    location ~ /.well-known/acme-challenge {
        proxy_pass http://127.0.0.1:9180;
    }
}
server {
    listen 443 ssl;
    listen [::]:443 ssl;
    server_name example.com;
    client_max_body_size 1000m;
    location / {
        proxy_set_header Host $host;
        proxy_pass http://127.0.0.1:9000;
    }
    location ~ /.well-known/acme-challenge {
        proxy_pass http://127.0.0.1:9180;
    }
}
`

const legacyServerReturnConfig = `
server {
    listen 80;
    server_name example.com www.example.com;
    return 301 https://$host$request_uri;
}
server {
    listen 443 ssl;
    server_name example.com www.example.com;
    return 301 https://$host$request_uri;
    root /var/www/example;
    index index.html;
    location / {
        try_files $uri $uri/ /index.html;
    }
}
`

const certifiedTLSConfig = `
server {
    listen 80;
    server_name example.com;
    location / {
        return 301 https://$host$request_uri;
    }
}
server {
    listen 443 ssl;
    server_name example.com;
    ssl_certificate /etc/nginx/ssl/old/fullchain.cer;
    ssl_certificate_key /etc/nginx/ssl/old/private.key;
    location / {
        proxy_pass http://127.0.0.1:9000;
    }
}
`

const customBlocksConfig = `
map $http_upgrade $connection_upgrade {
    default upgrade;
    '' close;
}
limit_req_zone $binary_remote_addr zone=perip:10m rate=5r/s;
upstream backend {
    server 127.0.0.1:8080;
    keepalive 16;
}
server {
    listen 80;
    server_name other.test;
    location / {
        return 301 https://$host$request_uri;
    }
}
server {
    listen 80;
    server_name example.com;
    location / {
        proxy_pass http://backend;
    }
}
`

func TestPlanHTTPS(t *testing.T) {
	tests := []struct {
		name   string
		config string
		req    HTTPSPlanRequest
		// siteEnabled marks a live site; the cases default to a new site that
		// is not enabled yet.
		siteEnabled bool
		check       func(t *testing.T, plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string)
	}{
		{
			name:   "quick setup with redirect serves the app on port 80 until issued",
			config: quickSetupRedirectConfig,
			req:    HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: true},
			check: func(t *testing.T, plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string) {
				assert.Equal(t, 1, plan.PendingTLSServers)
				assert.False(t, plan.HasCertifiedTLS)

				require.Len(t, staged.Servers, 1)
				assert.Empty(t, tlsServers(staged))
				port80 := staged.Servers[0]
				assertNoHTTPSRedirect(t, port80)
				assert.Equal(t, 1, challengeCount(port80))
				assert.Contains(t, stagedText, "proxy_pass http://127.0.0.1:9000")
				assert.Equal(t, []string{"1000m"}, directiveValues(port80, "client_max_body_size"))
				assert.Equal(t, []string{"80", "[::]:80"}, directiveValues(port80, "listen"))
				assert.NotContains(t, stagedText, "ssl_certificate")

				require.Len(t, final.Servers, 2)
				tls := tlsServers(final)
				require.Len(t, tls, 1)
				assert.Equal(t, []string{testCertPath}, directiveValues(tls[0], "ssl_certificate"))
				assert.Equal(t, []string{testKeyPath}, directiveValues(tls[0], "ssl_certificate_key"))
				assertNoHTTPSRedirect(t, tls[0])
				assert.Contains(t, serverText(t, tls[0]), "proxy_pass http://127.0.0.1:9000")
				assert.Equal(t, 1, challengeCount(tls[0]))

				plain := plainServers(final)
				require.Len(t, plain, 1)
				assertCanonicalRedirect(t, plain[0])
				assert.NotContains(t, serverText(t, plain[0]), "127.0.0.1:9000")
			},
		},
		{
			name:   "quick setup without redirect keeps the app on port 80 in final",
			config: quickSetupRedirectConfig,
			req:    HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01},
			check: func(t *testing.T, plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string) {
				plain := plainServers(final)
				require.Len(t, plain, 1)
				assertNoHTTPSRedirect(t, plain[0])
				assert.Contains(t, serverText(t, plain[0]), "proxy_pass http://127.0.0.1:9000")
				require.Len(t, tlsServers(final), 1)
			},
		},
		{
			name:   "legacy server-level return with two names never loops on TLS",
			config: legacyServerReturnConfig,
			req:    HTTPSPlanRequest{Domains: []string{"example.com", "www.example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: true},
			check: func(t *testing.T, plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string) {
				require.Len(t, staged.Servers, 1)
				port80 := staged.Servers[0]
				assertNoHTTPSRedirect(t, port80)
				assert.Empty(t, directiveValues(port80, "return"))
				assert.Equal(t, []string{"/var/www/example"}, directiveValues(port80, "root"))
				assert.Equal(t, 1, challengeCount(port80))

				tls := tlsServers(final)
				require.Len(t, tls, 1)
				assert.Empty(t, directiveValues(tls[0], "return"))
				assertNoHTTPSRedirect(t, tls[0])
				assert.Equal(t, []string{"example.com www.example.com"}, directiveValues(tls[0], "server_name"))
				assert.Equal(t, []string{testCertPath}, directiveValues(tls[0], "ssl_certificate"))
				assert.Equal(t, []string{"/var/www/example"}, directiveValues(tls[0], "root"))

				plain := plainServers(final)
				require.Len(t, plain, 1)
				assertCanonicalRedirect(t, plain[0])
			},
		},
		{
			name: "legacy redirect-only port 80 without TLS server generates a loop-free TLS server",
			config: `
server {
    listen 80;
    server_name example.com www.example.com;
    return 301 https://$host$request_uri;
}
`,
			req: HTTPSPlanRequest{Domains: []string{"example.com", "www.example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: true},
			check: func(t *testing.T, plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string) {
				assert.NotContains(t, stagedText, "https://$host")
				require.Len(t, plan.Diagnostics, 1)
				assert.Equal(t, HTTPSDiagnosticRedirectWithoutApp, plan.Diagnostics[0].Code)

				tls := tlsServers(final)
				require.Len(t, tls, 1)
				assertNoHTTPSRedirect(t, tls[0])
				assert.Equal(t, []string{"443 ssl", "[::]:443 ssl"}, directiveValues(tls[0], "listen"))
				assert.Equal(t, []string{"example.com www.example.com"}, directiveValues(tls[0], "server_name"))
				assertCanonicalRedirect(t, plainServers(final)[0])
			},
		},
		{
			name:   "existing TLS server with certificate is a reissue",
			config: certifiedTLSConfig,
			req:    HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: true},
			check: func(t *testing.T, plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string) {
				assert.True(t, plan.HasCertifiedTLS)
				assert.Zero(t, plan.PendingTLSServers)

				// The working TLS server stays and the redirect is kept; the
				// challenge is answered on both sides of the redirect.
				require.Len(t, staged.Servers, 2)
				assertCanonicalRedirect(t, plainServers(staged)[0])
				stagedTLS := tlsServers(staged)
				require.Len(t, stagedTLS, 1)
				assert.Equal(t, []string{"/etc/nginx/ssl/old/fullchain.cer"}, directiveValues(stagedTLS[0], "ssl_certificate"))
				assert.Equal(t, 1, challengeCount(stagedTLS[0]))

				require.Len(t, final.Servers, 2)
				tls := tlsServers(final)
				require.Len(t, tls, 1)
				assert.Equal(t, []string{testCertPath}, directiveValues(tls[0], "ssl_certificate"))
				assert.Equal(t, []string{testKeyPath}, directiveValues(tls[0], "ssl_certificate_key"))
				assert.Contains(t, serverText(t, tls[0]), "proxy_pass http://127.0.0.1:9000")
				assert.Equal(t, 1, challengeCount(tls[0]))
				assertCanonicalRedirect(t, plainServers(final)[0])
			},
		},
		{
			name:   "reissue without redirect request preserves the existing redirect",
			config: certifiedTLSConfig,
			req:    HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01},
			check: func(t *testing.T, plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string) {
				assertCanonicalRedirect(t, plainServers(final)[0])
				assert.Equal(t, []string{testCertPath}, directiveValues(tlsServers(final)[0], "ssl_certificate"))
			},
		},
		{
			name: "no port-80 server gets one that serves the pending app",
			config: `
server {
    listen 443 ssl;
    server_name example.com;
    root /srv/app;
}
`,
			req: HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01},
			check: func(t *testing.T, plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string) {
				assert.True(t, plan.AddedHTTPServer)
				require.Len(t, staged.Servers, 1)
				port80 := staged.Servers[0]
				assert.Equal(t, []string{"80", "[::]:80"}, directiveValues(port80, "listen"))
				assert.Equal(t, []string{"example.com"}, directiveValues(port80, "server_name"))
				assert.Equal(t, []string{"/srv/app"}, directiveValues(port80, "root"))
				assert.Equal(t, 1, challengeCount(port80))

				require.Len(t, final.Servers, 2)
				tls := tlsServers(final)
				require.Len(t, tls, 1)
				assert.Equal(t, []string{testCertPath}, directiveValues(tls[0], "ssl_certificate"))
			},
		},
		{
			name: "dns-01 injects no challenge and still stages without the pending TLS server",
			config: `
server {
    listen 80;
    server_name example.com;
    location / {
        return 301 https://$host$request_uri;
    }
}
server {
    listen 443 ssl;
    server_name example.com;
    location / {
        proxy_pass http://127.0.0.1:9000;
    }
}
`,
			req: HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeDNS01, RedirectHTTPToHTTPS: true},
			check: func(t *testing.T, plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string) {
				assert.NotContains(t, stagedText, "acme-challenge")
				assert.NotContains(t, finalText, "acme-challenge")
				require.Len(t, staged.Servers, 1)
				assertNoHTTPSRedirect(t, staged.Servers[0])
				assert.Contains(t, stagedText, "proxy_pass http://127.0.0.1:9000")

				tls := tlsServers(final)
				require.Len(t, tls, 1)
				assert.Equal(t, []string{testCertPath}, directiveValues(tls[0], "ssl_certificate"))
				assertNoHTTPSRedirect(t, tls[0])
				plain := plainServers(final)
				require.Len(t, plain, 1)
				content, ok := rootLocationContent(plain[0])
				require.True(t, ok)
				assert.True(t, contentRedirectsOnlyToHTTPS(content))
			},
		},
		{
			name:   "custom blocks, upstreams and unrelated servers are preserved",
			config: customBlocksConfig,
			req:    HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: true},
			check: func(t *testing.T, plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string) {
				for _, text := range []string{stagedText, finalText} {
					assert.Contains(t, text, "map $http_upgrade $connection_upgrade")
					assert.Contains(t, text, "limit_req_zone $binary_remote_addr zone=perip:10m rate=5r/s")
					assert.Contains(t, text, "upstream backend")
					assert.Contains(t, text, "keepalive 16")
				}
				// The unrelated server keeps its own redirect untouched.
				for _, cfg := range []*nginx.NgxConfig{staged, final} {
					other := cfg.Servers[0]
					assert.Equal(t, []string{"other.test"}, directiveValues(other, "server_name"))
					content, ok := rootLocationContent(other)
					require.True(t, ok)
					assert.True(t, contentRedirectsOnlyToHTTPS(content))
					assert.Equal(t, 0, challengeCount(other))
				}
				tls := tlsServers(final)
				require.Len(t, tls, 1)
				assert.Contains(t, serverText(t, tls[0]), "proxy_pass http://backend")
			},
		},
		{
			name: "domains missing from server_name are added",
			config: `
server {
    listen 80;
    server_name example.com;
    root /srv/app;
}
`,
			req: HTTPSPlanRequest{Domains: []string{"example.com", "www.example.com"}, ChallengeMethod: HTTPSChallengeHTTP01},
			check: func(t *testing.T, plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string) {
				require.Len(t, plan.Diagnostics, 1)
				assert.Equal(t, HTTPSDiagnosticServerNamesExtended, plan.Diagnostics[0].Code)
				assert.Equal(t, []string{"example.com www.example.com"}, directiveValues(staged.Servers[0], "server_name"))
				assert.Equal(t, []string{"example.com www.example.com"}, directiveValues(tlsServers(final)[0], "server_name"))
			},
		},
		{
			name: "server-level return that is not an HTTPS redirect moves into location /",
			config: `
server {
    listen 80;
    server_name example.com;
    return 302 https://app.example.net$request_uri;
}
`,
			req: HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01},
			check: func(t *testing.T, plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string) {
				port80 := staged.Servers[0]
				assert.Empty(t, directiveValues(port80, "return"))
				content, ok := rootLocationContent(port80)
				require.True(t, ok)
				assert.Contains(t, content, "return 302 https://app.example.net$request_uri;")
				assert.Equal(t, 1, challengeCount(port80))
			},
		},
		{
			name: "duplicate challenge locations are replaced by one",
			config: `
server {
    listen 80;
    server_name example.com;
    location ^~ /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }
    location ~ /.well-known/acme-challenge {
        proxy_pass http://127.0.0.1:1;
    }
    root /srv/app;
}
`,
			req: HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01},
			check: func(t *testing.T, plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string) {
				assert.Equal(t, 1, challengeCount(staged.Servers[0]))
				assert.Contains(t, stagedText, "127.0.0.1:9180")
				assert.NotContains(t, stagedText, "certbot")
			},
		},
		{
			name: "server-level if redirect is stripped while TLS is pending",
			config: `
server {
    listen 80;
    listen 443 ssl;
    server_name example.com;
    if ($scheme = http) {
        return 301 https://$host$request_uri;
    }
    root /srv/app;
}
`,
			req: HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01},
			check: func(t *testing.T, plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string) {
				assert.Equal(t, 1, plan.PendingTLSServers)
				require.Len(t, staged.Servers, 1)
				assert.NotContains(t, stagedText, "https://$host")
				assert.NotContains(t, stagedText, "443")
				assert.Equal(t, 1, challengeCount(staged.Servers[0]))

				// The server has content of its own, so it is not a redirect-only
				// server waiting for app content.
				assert.Empty(t, plan.Diagnostics)

				// Without a redirect request the combined server is kept as one
				// block; its scheme-guarded redirect cannot loop and stays.
				require.Len(t, final.Servers, 1)
				mixed := final.Servers[0]
				assert.Equal(t, []string{"80", "443 ssl"}, directiveValues(mixed, "listen"))
				assert.Equal(t, []string{testCertPath}, directiveValues(mixed, "ssl_certificate"))
				assert.Empty(t, directiveValues(mixed, "return"))
				assert.Contains(t, serverText(t, mixed), "if ($scheme = http)")
				assert.Equal(t, 1, strings.Count(finalText, "return 301 https://$host$request_uri;"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mustParseNgxConfig(t, tt.config)
			before, err := cfg.BuildConfig()
			require.NoError(t, err)

			req := tt.req
			req.ChallengeLocation = testChallengeLocation
			req.SiteDisabled = !tt.siteEnabled
			plan, err := PlanHTTPS(cfg, req)
			require.NoError(t, err)

			finalCfg, err := plan.BuildFinal(testCertPath, testKeyPath)
			require.NoError(t, err)

			stagedText, staged := rebuild(t, plan.Staged)
			finalText, final := rebuild(t, finalCfg)

			// Planning is pure: the input and the staged plan stay untouched.
			after, err := cfg.BuildConfig()
			require.NoError(t, err)
			assert.Equal(t, before, after)
			stagedAgain, err := plan.Staged.BuildConfig()
			require.NoError(t, err)
			assert.Equal(t, stagedText, stagedAgain)

			tt.check(t, plan, staged, final, stagedText, finalText)
		})
	}
}

func TestPlanHTTPSRejectsInvalidRequests(t *testing.T) {
	cfg := mustParseNgxConfig(t, `server { listen 80; server_name example.com; }`)

	_, err := PlanHTTPS(cfg, HTTPSPlanRequest{ChallengeMethod: HTTPSChallengeHTTP01, ChallengeLocation: testChallengeLocation})
	assert.ErrorIs(t, err, ErrHTTPSNoDomains)

	_, err = PlanHTTPS(cfg, HTTPSPlanRequest{Domains: []string{"example.com; return 200"}, ChallengeLocation: testChallengeLocation})
	assert.ErrorIs(t, err, ErrHTTPSInvalidDomain)

	_, err = PlanHTTPS(cfg, HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: "tls-alpn", ChallengeLocation: testChallengeLocation})
	assert.ErrorIs(t, err, ErrHTTPSInvalidChallenge)

	stream := mustParseNgxConfig(t, `stream { server { listen 12345; proxy_pass 127.0.0.1:22; } }`)
	_, err = PlanHTTPS(stream, HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeLocation: testChallengeLocation})
	assert.ErrorIs(t, err, ErrHTTPSStreamConfig)

	plan, err := PlanHTTPS(cfg, HTTPSPlanRequest{Domains: []string{" Example.COM. ", "example.com"}, ChallengeLocation: testChallengeLocation})
	require.NoError(t, err)
	assert.Equal(t, []string{"example.com"}, plan.Request.Domains)
	assert.Equal(t, HTTPSChallengeHTTP01, plan.Request.ChallengeMethod)

	_, err = plan.BuildFinal("", testKeyPath)
	assert.ErrorIs(t, err, ErrHTTPSMissingCertificatePaths)
}

func TestPlanHTTPSUsesLetsEncryptTemplate(t *testing.T) {
	setupMaintenanceTestSettings(t, "")
	cfg := mustParseNgxConfig(t, `server { listen 80; server_name example.com; root /srv/app; }`)

	plan, err := PlanHTTPS(cfg, HTTPSPlanRequest{Domains: []string{"example.com"}})
	require.NoError(t, err)

	content, err := plan.Staged.BuildConfig()
	require.NoError(t, err)
	assert.Contains(t, content, "/.well-known/acme-challenge")
	assert.Contains(t, content, "127.0.0.1:9180")
}

func TestServerNameMatches(t *testing.T) {
	tests := []struct {
		name, domain string
		want         bool
	}{
		{"example.com", "example.com", true},
		{"Example.COM", "example.com", true},
		{"*.example.com", "www.example.com", true},
		{"*.example.com", "example.com", false},
		{".example.com", "example.com", true},
		{".example.com", "a.b.example.com", true},
		{"www.*", "www.example.com", true},
		{`~^(www\.)?example\.com$`, "www.example.com", true},
		{`~^api\.`, "www.example.com", false},
		{"_", "example.com", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, serverNameMatches(tt.name, tt.domain), "%s vs %s", tt.name, tt.domain)
	}
}

func TestListenKinds(t *testing.T) {
	tests := []struct {
		listens     []string
		tls, plain8 bool
	}{
		{[]string{"80"}, false, true},
		{[]string{"[::]:80 default_server"}, false, true},
		{[]string{"127.0.0.1:80"}, false, true},
		{[]string{"127.0.0.1"}, false, true},
		{[]string{"8080"}, false, false},
		{[]string{"443 ssl http2"}, true, false},
		{[]string{"443 quic reuseport"}, false, false},
		{nil, false, true},
	}
	for _, tt := range tests {
		server := nginx.NewNgxServer()
		for _, listen := range tt.listens {
			server.Directives = append(server.Directives, &nginx.NgxDirective{Directive: "listen", Params: listen})
		}
		tls, plain := serverListenKinds(server)
		assert.Equal(t, tt.tls, tls, strings.Join(tt.listens, ","))
		assert.Equal(t, tt.plain8, plain, strings.Join(tt.listens, ","))
	}
}

func TestRoutesHTTP01Challenge(t *testing.T) {
	routed := mustParseNgxConfig(t, quickSetupRedirectConfig)
	assert.True(t, RoutesHTTP01Challenge(routed, []string{"example.com"}))
	assert.False(t, RoutesHTTP01Challenge(routed, []string{"example.com", "www.example.com"}))

	serverReturn := mustParseNgxConfig(t, legacyServerReturnConfig)
	assert.False(t, RoutesHTTP01Challenge(serverReturn, []string{"example.com"}))

	plan, err := PlanHTTPS(serverReturn, HTTPSPlanRequest{Domains: []string{"example.com"}, SiteDisabled: true, ChallengeLocation: testChallengeLocation})
	require.NoError(t, err)
	assert.True(t, RoutesHTTP01Challenge(plan.Staged, []string{"example.com"}))
	assert.False(t, plan.StagedListensIPv6())

	quick, err := PlanHTTPS(routed, HTTPSPlanRequest{Domains: []string{"example.com"}, SiteDisabled: true, ChallengeLocation: testChallengeLocation})
	require.NoError(t, err)
	assert.True(t, quick.StagedListensIPv6())
}
