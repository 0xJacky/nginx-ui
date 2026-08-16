package template

import (
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/stretchr/testify/assert"
)

func TestBuildQuickConfigReverseProxy(t *testing.T) {
	req := QuickConfigRequest{
		Type:                QuickConfigTypeReverseProxy,
		Domains:             []string{"example.com", "www.example.com"},
		EnableWebSocket:     true,
		ClientMaxBodySize:   "100m",
		Scheme:              "http",
		Host:                "127.0.0.1",
		Port:                "8080",
		EnableTLS:           true,
		RedirectHTTPToHTTPS: true,
	}
	req.fillDefaults()
	assert.NoError(t, req.validate())

	cfg, err := buildQuickConfig(&req)
	assert.NoError(t, err)
	assert.Len(t, cfg.Servers, 2)
	assert.Equal(t, "example.com", cfg.Name)

	port80 := cfg.Servers[0]
	// HTTP -> HTTPS redirect
	returnDirectives := findServerDirectives(port80, "return")
	if assert.Len(t, returnDirectives, 1) {
		assert.Equal(t, "301 https://$host$request_uri", returnDirectives[0].Params)
	}
	// Challenge location must be available on port 80 for HTTP-01.
	assertLocation(t, port80, "~ /.well-known/acme-challenge")

	tls := cfg.Servers[1]
	assert.Equal(t, "443 ssl", findServerDirectives(tls, "listen")[0].Params)
	assert.Equal(t, "example.com www.example.com", findServerDirectives(tls, "server_name")[0].Params)
	// Empty ssl_certificate placeholders, filled in by the cert flow.
	assert.Empty(t, findServerDirectives(tls, "ssl_certificate")[0].Params)
	assert.Empty(t, findServerDirectives(tls, "ssl_certificate_key")[0].Params)

	// Reverse proxy location / must proxy to the target.
	location := findLocation(tls, "/")
	assert.Contains(t, location.Content, "proxy_pass http://127.0.0.1:8080/;")
	assert.Contains(t, location.Content, "client_max_body_size 100m;")
	// WebSocket upgrade headers.
	assert.Contains(t, location.Content, "proxy_set_header Upgrade $http_upgrade;")
	assert.Contains(t, cfg.Custom, "map $http_upgrade $connection_upgrade")

	// The built config must be re-parseable.
	content, err := cfg.BuildConfig()
	assert.NoError(t, err)
	assert.NotEmpty(t, content)
	parsed, err := nginx.ParseNgxConfigByContent(content)
	assert.NoError(t, err)
	assert.Len(t, parsed.Servers, 2)
}

func TestBuildQuickConfigReverseProxyNoTLS(t *testing.T) {
	req := QuickConfigRequest{
		Type:              QuickConfigTypeReverseProxy,
		Domains:           []string{"example.com"},
		Scheme:            "https",
		Host:              "10.0.0.5",
		Port:              "8443",
		EnableWebSocket:   false,
		ClientMaxBodySize: "10m",
	}
	req.fillDefaults()
	assert.NoError(t, req.validate())

	cfg, err := buildQuickConfig(&req)
	assert.NoError(t, err)
	assert.Len(t, cfg.Servers, 1)
	// Only the proxy location, no challenge location when TLS is disabled.
	assertLocation(t, cfg.Servers[0], "/")
	assert.Nil(t, findLocation(cfg.Servers[0], "~ /.well-known/acme-challenge"))

	location := findLocation(cfg.Servers[0], "/")
	assert.Contains(t, location.Content, "proxy_pass https://10.0.0.5:8443/;")
	// WebSocket headers only rendered when enabled.
	assert.NotContains(t, location.Content, "proxy_set_header Upgrade $http_upgrade;")
	// The Forwarded maps must stay even without WebSocket because the
	// unconditional proxy_set_header Forwarded line depends on them.
	assert.Contains(t, cfg.Custom, "map $http_forwarded $proxy_add_forwarded")
	assert.Contains(t, cfg.Custom, "map $remote_addr $proxy_forwarded_elem")
	assert.NotContains(t, cfg.Custom, "map $http_upgrade $connection_upgrade")
}

func TestBuildQuickConfigStatic(t *testing.T) {
	req := QuickConfigRequest{
		Type:        QuickConfigTypeStatic,
		Domains:     []string{"static.example.com"},
		WebRoot:     "/var/www/html",
		SpaFallback: true,
	}
	req.fillDefaults()
	assert.NoError(t, req.validate())

	cfg, err := buildQuickConfig(&req)
	assert.NoError(t, err)
	assert.Len(t, cfg.Servers, 1)
	assert.Equal(t, "/var/www/html", findServerDirectives(cfg.Servers[0], "root")[0].Params)
	assert.Equal(t, "index.html", findServerDirectives(cfg.Servers[0], "index")[0].Params)
	assertLocation(t, cfg.Servers[0], "/")

	content, err := cfg.BuildConfig()
	assert.NoError(t, err)
	assert.Contains(t, content, "root /var/www/html;")
	assert.Contains(t, content, "try_files $uri $uri/ /index.html;")

	_, err = nginx.ParseNgxConfigByContent(content)
	assert.NoError(t, err)
}

func TestBuildQuickConfigStaticTLSWithoutRedirect(t *testing.T) {
	req := QuickConfigRequest{
		Type:                QuickConfigTypeStatic,
		Domains:             []string{"static.example.com"},
		WebRoot:             "/srv/www",
		EnableTLS:           true,
		RedirectHTTPToHTTPS: false,
	}
	req.fillDefaults()
	assert.NoError(t, req.validate())

	cfg, err := buildQuickConfig(&req)
	assert.NoError(t, err)
	assert.Len(t, cfg.Servers, 2)
	// Port 80 keeps serving the site and exposes the challenge location.
	assert.NotNil(t, findServerDirectives(cfg.Servers[0], "root"))
	assertLocation(t, cfg.Servers[0], "~ /.well-known/acme-challenge")
	assertLocation(t, cfg.Servers[1], "~ /.well-known/acme-challenge")
}

func TestBuildQuickConfigRedirect(t *testing.T) {
	req := QuickConfigRequest{
		Type:           QuickConfigTypeRedirect,
		Domains:        []string{"old.example.com"},
		TargetURL:      "https://new.example.com",
		RedirectStatus: "308",
	}
	req.fillDefaults()
	assert.NoError(t, req.validate())

	cfg, err := buildQuickConfig(&req)
	assert.NoError(t, err)
	assert.Len(t, cfg.Servers, 1)

	location := findLocation(cfg.Servers[0], "/")
	assert.Contains(t, location.Content, "return 308 https://new.example.com;")
	content, err := cfg.BuildConfig()
	assert.NoError(t, err)
	assert.Contains(t, content, "return 308 https://new.example.com;")
	_, err = nginx.ParseNgxConfigByContent(content)
	assert.NoError(t, err)
}

func TestBuildQuickConfigValidation(t *testing.T) {
	t.Run("missing domains", func(t *testing.T) {
		req := QuickConfigRequest{Type: QuickConfigTypeStatic, WebRoot: "/var/www"}
		assert.Error(t, req.validate())
	})

	t.Run("invalid scheme", func(t *testing.T) {
		req := QuickConfigRequest{Type: QuickConfigTypeReverseProxy, Domains: []string{"a.com"}, Scheme: "ftp"}
		assert.Error(t, req.validate())
	})

	t.Run("static missing web_root", func(t *testing.T) {
		req := QuickConfigRequest{Type: QuickConfigTypeStatic, Domains: []string{"a.com"}}
		assert.Error(t, req.validate())
	})

	t.Run("redirect missing target", func(t *testing.T) {
		req := QuickConfigRequest{Type: QuickConfigTypeRedirect, Domains: []string{"a.com"}}
		assert.Error(t, req.validate())
	})

	t.Run("defaults applied", func(t *testing.T) {
		req := QuickConfigRequest{Type: QuickConfigTypeRedirect, Domains: []string{"a.com"}, TargetURL: "https://b.com"}
		req.fillDefaults()
		assert.Equal(t, "301", req.RedirectStatus)
	})

	tests := []struct {
		name string
		req  QuickConfigRequest
	}{
		{
			name: "domain directive injection",
			req:  QuickConfigRequest{Type: QuickConfigTypeStatic, Domains: []string{"example.com;\nreturn 200"}, WebRoot: "/var/www"},
		},
		{
			name: "reverse proxy host directive injection",
			req:  QuickConfigRequest{Type: QuickConfigTypeReverseProxy, Domains: []string{"example.com"}, Host: "127.0.0.1;", Port: "9000"},
		},
		{
			name: "reverse proxy non-numeric port",
			req:  QuickConfigRequest{Type: QuickConfigTypeReverseProxy, Domains: []string{"example.com"}, Host: "127.0.0.1", Port: "http"},
		},
		{
			name: "reverse proxy out-of-range port",
			req:  QuickConfigRequest{Type: QuickConfigTypeReverseProxy, Domains: []string{"example.com"}, Host: "127.0.0.1", Port: "65536"},
		},
		{
			name: "reverse proxy invalid body size",
			req:  QuickConfigRequest{Type: QuickConfigTypeReverseProxy, Domains: []string{"example.com"}, Host: "127.0.0.1", Port: "9000", ClientMaxBodySize: "100m;"},
		},
		{
			name: "static root directive injection",
			req:  QuickConfigRequest{Type: QuickConfigTypeStatic, Domains: []string{"example.com"}, WebRoot: "/var/www;\nreturn 200"},
		},
		{
			name: "static index directive injection",
			req:  QuickConfigRequest{Type: QuickConfigTypeStatic, Domains: []string{"example.com"}, WebRoot: "/var/www", Index: "index.html;"},
		},
		{
			name: "redirect target directive injection",
			req:  QuickConfigRequest{Type: QuickConfigTypeRedirect, Domains: []string{"example.com"}, TargetURL: "https://new.example.com;return", RedirectStatus: "301"},
		},
		{
			name: "redirect non-http target",
			req:  QuickConfigRequest{Type: QuickConfigTypeRedirect, Domains: []string{"example.com"}, TargetURL: "javascript:alert(1)", RedirectStatus: "301"},
		},
		{
			name: "redirect unsupported status",
			req:  QuickConfigRequest{Type: QuickConfigTypeRedirect, Domains: []string{"example.com"}, TargetURL: "https://new.example.com", RedirectStatus: "307"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.fillDefaults()
			assert.Error(t, tt.req.validate())
		})
	}

	t.Run("bracketed IPv6 reverse proxy host", func(t *testing.T) {
		req := QuickConfigRequest{
			Type:    QuickConfigTypeReverseProxy,
			Domains: []string{"example.com"},
			Host:    "[::1]",
			Port:    "9000",
		}
		req.fillDefaults()
		assert.NoError(t, req.validate())
	})
}

func assertQuickConfigEqual(t *testing.T, expected, actual QuickConfigRequest) {
	t.Helper()
	assert.Equal(t, expected.Type, actual.Type)
	assert.Equal(t, expected.Domains, actual.Domains)
	assert.Equal(t, expected.EnableTLS, actual.EnableTLS)
	assert.Equal(t, expected.RedirectHTTPToHTTPS, actual.RedirectHTTPToHTTPS)
	assert.Equal(t, expected.Scheme, actual.Scheme)
	assert.Equal(t, expected.Host, actual.Host)
	assert.Equal(t, expected.Port, actual.Port)
	assert.Equal(t, expected.EnableWebSocket, actual.EnableWebSocket)
	assert.Equal(t, expected.ClientMaxBodySize, actual.ClientMaxBodySize)
	assert.Equal(t, expected.WebRoot, actual.WebRoot)
	assert.Equal(t, expected.Index, actual.Index)
	assert.Equal(t, expected.SpaFallback, actual.SpaFallback)
	assert.Equal(t, expected.TargetURL, actual.TargetURL)
	assert.Equal(t, expected.RedirectStatus, actual.RedirectStatus)
}

// assertAnalyzeRoundTrip builds a config from the request, re-parses it and
// asserts that analysis recovers the exact same request. This guarantees the
// edit-page pre-fill mirrors what the wizard generated.
func assertAnalyzeRoundTrip(t *testing.T, req QuickConfigRequest) {
	t.Helper()
	req.fillDefaults()
	assert.NoError(t, req.validate())

	cfg, err := buildQuickConfig(&req)
	assert.NoError(t, err)
	content, err := cfg.BuildConfig()
	assert.NoError(t, err)

	parsed, err := nginx.ParseNgxConfigByContent(content)
	assert.NoError(t, err)

	analyzed := analyzeNgxConfig(parsed)
	assertQuickConfigEqual(t, req, analyzed)
}

func TestAnalyzeQuickConfigRoundTrip(t *testing.T) {
	t.Run("reverse proxy with TLS and redirect", func(t *testing.T) {
		assertAnalyzeRoundTrip(t, QuickConfigRequest{
			Type:                QuickConfigTypeReverseProxy,
			Domains:             []string{"example.com", "www.example.com"},
			EnableWebSocket:     true,
			ClientMaxBodySize:   "100m",
			Scheme:              "http",
			Host:                "127.0.0.1",
			Port:                "8080",
			EnableTLS:           true,
			RedirectHTTPToHTTPS: true,
		})
	})

	t.Run("reverse proxy no TLS", func(t *testing.T) {
		assertAnalyzeRoundTrip(t, QuickConfigRequest{
			Type:              QuickConfigTypeReverseProxy,
			Domains:           []string{"example.com"},
			Scheme:            "https",
			Host:              "10.0.0.5",
			Port:              "8443",
			EnableWebSocket:   false,
			ClientMaxBodySize: "10m",
		})
	})

	t.Run("static with SPA", func(t *testing.T) {
		assertAnalyzeRoundTrip(t, QuickConfigRequest{
			Type:        QuickConfigTypeStatic,
			Domains:     []string{"static.example.com"},
			WebRoot:     "/var/www/html",
			SpaFallback: true,
		})
	})

	t.Run("static TLS without redirect", func(t *testing.T) {
		assertAnalyzeRoundTrip(t, QuickConfigRequest{
			Type:                QuickConfigTypeStatic,
			Domains:             []string{"static.example.com"},
			WebRoot:             "/srv/www",
			EnableTLS:           true,
			RedirectHTTPToHTTPS: false,
		})
	})

	t.Run("redirect", func(t *testing.T) {
		assertAnalyzeRoundTrip(t, QuickConfigRequest{
			Type:           QuickConfigTypeRedirect,
			Domains:        []string{"old.example.com"},
			TargetURL:      "https://new.example.com",
			RedirectStatus: "308",
		})
	})

	t.Run("redirect with TLS and redirect to https", func(t *testing.T) {
		assertAnalyzeRoundTrip(t, QuickConfigRequest{
			Type:                QuickConfigTypeRedirect,
			Domains:             []string{"old.example.com"},
			TargetURL:           "https://new.example.com",
			RedirectStatus:      "301",
			EnableTLS:           true,
			RedirectHTTPToHTTPS: true,
		})
	})
}

func TestAnalyzeQuickConfigBestEffort(t *testing.T) {
	t.Run("unrecognized config falls back to defaults", func(t *testing.T) {
		cfg, err := buildQuickConfig(&QuickConfigRequest{
			Type:    QuickConfigTypeStatic,
			Domains: []string{"a.com"},
			WebRoot: "/tmp/site",
		})
		assert.NoError(t, err)

		var kept []*nginx.NgxDirective
		for _, d := range cfg.Servers[0].Directives {
			if d.Directive == "root" || d.Directive == "index" {
				continue
			}
			kept = append(kept, d)
		}
		cfg.Servers[0].Directives = kept

		req := analyzeNgxConfig(cfg)
		assert.Equal(t, QuickConfigTypeReverseProxy, req.Type)
		assert.Equal(t, []string{"a.com"}, req.Domains)
	})

	t.Run("server name default underscore is ignored", func(t *testing.T) {
		req := QuickConfigRequest{
			Type:    QuickConfigTypeStatic,
			Domains: []string{"a.com"},
			WebRoot: "/tmp/site",
		}
		req.fillDefaults()
		cfg, err := buildQuickConfig(&req)
		assert.NoError(t, err)
		cfg.Servers[0].Directives = append(cfg.Servers[0].Directives,
			&nginx.NgxDirective{Directive: "listen", Params: "80 default_server"})
		cfg.Servers[0].Directives = append(cfg.Servers[0].Directives,
			&nginx.NgxDirective{Directive: "server_name", Params: "_"})

		analyzed := analyzeNgxConfig(cfg)
		assert.Equal(t, []string{"a.com"}, analyzed.Domains)
	})
}

func findServerDirectives(server *nginx.NgxServer, name string) []*nginx.NgxDirective {
	var result []*nginx.NgxDirective
	for _, d := range server.Directives {
		if d.Directive == name {
			result = append(result, d)
		}
	}
	return result
}

func findLocation(server *nginx.NgxServer, path string) *nginx.NgxLocation {
	for _, l := range server.Locations {
		if strings.TrimSpace(l.Path) == path {
			return l
		}
	}
	return nil
}

func assertLocation(t *testing.T, server *nginx.NgxServer, path string) {
	t.Helper()
	l := findLocation(server, path)
	if assert.NotNil(t, l, "expected location %q", path) {
		assert.NotEmpty(t, l.Content)
	}
}
