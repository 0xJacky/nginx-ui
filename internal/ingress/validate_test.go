package ingress

import (
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	setupRender(t, true)

	tests := []struct {
		name   string
		modify func(s *model.IngressSpec)
		want   error
	}{
		{name: "valid", modify: func(*model.IngressSpec) {}},
		{name: "no hosts", modify: func(s *model.IngressSpec) { s.Hosts = nil }, want: ErrInvalidHost},
		{name: "host with space", modify: func(s *model.IngressSpec) { s.Hosts = []string{"a b.example.com"} }, want: ErrInvalidHost},
		{name: "host with semicolon", modify: func(s *model.IngressSpec) { s.Hosts = []string{"a.example.com;"} }, want: ErrInvalidHost},
		{name: "single label host", modify: func(s *model.IngressSpec) { s.Hosts = []string{"localhost"} }, want: ErrInvalidHost},
		{name: "ip host", modify: func(s *model.IngressSpec) { s.Hosts = []string{"10.0.0.1"} }, want: ErrInvalidHost},
		{name: "underscore host", modify: func(s *model.IngressSpec) { s.Hosts = []string{"a_b.example.com"} }, want: ErrInvalidHost},
		{name: "duplicate host after idna", modify: func(s *model.IngressSpec) {
			s.Hosts = []string{"bücher.example.com", "xn--bcher-kva.example.com"}
		}, want: ErrInvalidHost},
		{name: "idn host", modify: func(s *model.IngressSpec) { s.Hosts = []string{"例子。测试"} }},
		{name: "double hyphen label", modify: func(s *model.IngressSpec) { s.Hosts = []string{"ab--cd.example.com"} }},
		{name: "wildcard with none", modify: func(s *model.IngressSpec) { s.Hosts = []string{"*.example.com"} }},
		{name: "wildcard with acme http01", modify: func(s *model.IngressSpec) {
			s.Hosts = []string{"*.example.com"}
			s.TLS = model.IngressTLS{Mode: model.IngressTLSModeACME}
		}, want: ErrInvalidHost},
		{name: "wildcard with acme dns01", modify: func(s *model.IngressSpec) {
			s.Hosts = []string{"*.example.com"}
			s.TLS = model.IngressTLS{Mode: model.IngressTLSModeACME, Challenge: model.IngressChallengeDNS01, DNSCredentialID: 1}
		}},
		{name: "wildcard in the middle", modify: func(s *model.IngressSpec) { s.Hosts = []string{"a.*.example.com"} }, want: ErrInvalidHost},
		{name: "no backends", modify: func(s *model.IngressSpec) { s.Backends = nil }, want: ErrInvalidBackend},
		{name: "bad backend name", modify: func(s *model.IngressSpec) {
			s.Backends[0].Name = "App"
			s.Routes[0].Backend = "App"
		}, want: ErrInvalidBackend},
		{name: "duplicate backend", modify: func(s *model.IngressSpec) { s.Backends = append(s.Backends, s.Backends[0]) }, want: ErrInvalidBackend},
		{name: "bad lb", modify: func(s *model.IngressSpec) { s.Backends[0].LB = "random" }, want: ErrInvalidBackend},
		{name: "no servers", modify: func(s *model.IngressSpec) { s.Backends[0].Servers = nil }, want: ErrInvalidBackend},
		{name: "hostname server", modify: func(s *model.IngressSpec) {
			s.Backends[0].Servers[0].Address = "localhost:8080"
		}, want: ErrInvalidBackend},
		{name: "server without port", modify: func(s *model.IngressSpec) {
			s.Backends[0].Servers[0].Address = "127.0.0.1"
		}, want: ErrInvalidBackend},
		{name: "server outside allowed range", modify: func(s *model.IngressSpec) {
			s.Backends[0].Servers[0].Address = "10.0.0.1:80"
		}, want: ErrBackendNotAllowed},
		{name: "ipv6 loopback server", modify: func(s *model.IngressSpec) {
			s.Backends[0].Servers[0].Address = "[::1]:8080"
		}},
		{name: "mapped ipv4 loopback server", modify: func(s *model.IngressSpec) {
			s.Backends[0].Servers[0].Address = "[::ffff:127.0.0.1]:8080"
		}},
		{name: "duplicate server", modify: func(s *model.IngressSpec) {
			s.Backends[0].Servers = append(s.Backends[0].Servers, s.Backends[0].Servers[0])
		}, want: ErrInvalidBackend},
		{name: "bad fail timeout", modify: func(s *model.IngressSpec) {
			s.Backends[0].Servers[0].FailTimeout = "10 s"
		}, want: ErrInvalidBackend},
		{name: "backup with ip_hash", modify: func(s *model.IngressSpec) {
			s.Backends[0].LB = model.IngressLBIPHash
			s.Backends[0].Servers[0].Backup = true
		}, want: ErrInvalidBackend},
		{name: "negative keepalive", modify: func(s *model.IngressSpec) { s.Backends[0].Keepalive = -1 }, want: ErrInvalidBackend},
		{name: "no routes", modify: func(s *model.IngressSpec) { s.Routes = nil }, want: ErrInvalidRoute},
		{name: "relative prefix", modify: func(s *model.IngressSpec) { s.Routes[0].PathPrefix = "api" }, want: ErrInvalidRoute},
		{name: "prefix with brace", modify: func(s *model.IngressSpec) { s.Routes[0].PathPrefix = "/a{" }, want: ErrInvalidRoute},
		{name: "prefix with quote", modify: func(s *model.IngressSpec) { s.Routes[0].PathPrefix = `/a"` }, want: ErrInvalidRoute},
		{name: "prefix with hash", modify: func(s *model.IngressSpec) { s.Routes[0].PathPrefix = "/a#b" }, want: ErrInvalidRoute},
		{name: "prefix with space", modify: func(s *model.IngressSpec) { s.Routes[0].PathPrefix = "/a b" }, want: ErrInvalidRoute},
		{name: "prefix on acme challenge", modify: func(s *model.IngressSpec) {
			s.Routes[0].PathPrefix = "/.well-known/acme-challenge/x"
		}, want: ErrInvalidRoute},
		{name: "duplicate prefix", modify: func(s *model.IngressSpec) { s.Routes = append(s.Routes, s.Routes[0]) }, want: ErrInvalidRoute},
		{name: "unknown backend", modify: func(s *model.IngressSpec) { s.Routes[0].Backend = "nope" }, want: ErrInvalidRoute},
		{name: "bad timeout", modify: func(s *model.IngressSpec) { s.Routes[0].ReadTimeout = "1 minute" }, want: ErrInvalidRoute},
		{name: "bad size", modify: func(s *model.IngressSpec) { s.Routes[0].ClientMaxBodySize = "64mb" }, want: ErrInvalidRoute},
		{name: "bad route cidr", modify: func(s *model.IngressSpec) { s.Routes[0].AllowCIDRs = []string{"10.0.0.0/33"} }, want: ErrInvalidRoute},
		{name: "empty tls mode", modify: func(s *model.IngressSpec) { s.TLS.Mode = "" }, want: ErrInvalidTLS},
		{name: "cert without id", modify: func(s *model.IngressSpec) { s.TLS = model.IngressTLS{Mode: model.IngressTLSModeCert} }, want: ErrInvalidTLS},
		{name: "dns01 without credential", modify: func(s *model.IngressSpec) {
			s.TLS = model.IngressTLS{Mode: model.IngressTLSModeACME, Challenge: model.IngressChallengeDNS01}
		}, want: ErrInvalidTLS},
		{name: "bad challenge", modify: func(s *model.IngressSpec) {
			s.TLS = model.IngressTLS{Mode: model.IngressTLSModeACME, Challenge: "tls-alpn"}
		}, want: ErrInvalidTLS},
		{name: "bad key type", modify: func(s *model.IngressSpec) {
			s.TLS = model.IngressTLS{Mode: model.IngressTLSModeACME, KeyType: "1024"}
		}, want: ErrInvalidTLS},
		{name: "bad policy cidr", modify: func(s *model.IngressSpec) { s.Policy.AllowCIDRs = []string{"nope"} }, want: ErrInvalidPolicy},
		{name: "rps zero", modify: func(s *model.IngressSpec) { s.Policy.RateLimit = &model.IngressRateLimit{RPS: 0} }, want: ErrInvalidPolicy},
		{name: "burst too large", modify: func(s *model.IngressSpec) {
			s.Policy.RateLimit = &model.IngressRateLimit{RPS: 1, Burst: 100001}
		}, want: ErrInvalidPolicy},
		{name: "bad policy size", modify: func(s *model.IngressSpec) { s.Policy.ClientMaxBodySize = "-1" }, want: ErrInvalidPolicy},
		{name: "bad header name", modify: func(s *model.IngressSpec) {
			s.Policy.ExtraHeaders = []model.IngressHeaderSet{{Name: "X Bad", Value: "v"}}
		}, want: ErrInvalidPolicy},
		{name: "reserved header", modify: func(s *model.IngressSpec) {
			s.Policy.ExtraHeaders = []model.IngressHeaderSet{{Name: "host", Value: "evil"}}
		}, want: ErrInvalidPolicy},
		{name: "header value with brace", modify: func(s *model.IngressSpec) {
			s.Policy.ExtraHeaders = []model.IngressHeaderSet{{Name: "X-A", Value: "a}"}}
		}, want: ErrInvalidPolicy},
		{name: "header value with newline", modify: func(s *model.IngressSpec) {
			s.Policy.ExtraHeaders = []model.IngressHeaderSet{{Name: "X-A", Value: "a\nb"}}
		}, want: ErrInvalidPolicy},
		{name: "header value with quote", modify: func(s *model.IngressSpec) {
			s.Policy.ExtraHeaders = []model.IngressHeaderSet{{Name: "X-A", Value: `a"`}}
		}, want: ErrInvalidPolicy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := basicSpec("app.example.com")
			tt.modify(&spec)
			err := Validate(&spec)
			if tt.want == nil {
				require.NoError(t, err)
				return
			}
			requireCode(t, err, tt.want)
		})
	}
}

func TestValidateUsesAllowedBackendsSetting(t *testing.T) {
	setupRender(t, true)
	spec := basicSpec("app.example.com")
	spec.Backends[0].Servers[0].Address = "10.1.2.3:80"
	requireCode(t, Validate(&spec), ErrBackendNotAllowed)

	settingsBackends(t, []string{"10.0.0.0/8", "not-a-cidr"})
	require.NoError(t, Validate(&spec))
}

func TestHostToASCII(t *testing.T) {
	tests := map[string]string{
		"Example.COM":          "example.com",
		"例子。测试":                "xn--fsqu00a.xn--0zwm56d",
		"例子.测试.":               "xn--fsqu00a.xn--0zwm56d",
		"*.Bücher.example.com": "*.xn--bcher-kva.example.com",
		"ab--cd.example.com":   "ab--cd.example.com",
	}
	for input, want := range tests {
		got, err := HostToASCII(input)
		require.NoError(t, err, input)
		assert.Equal(t, want, got, input)
	}

	for _, input := range []string{"", " a.example.com", "*", "*.com.*", "a..example.com", strings.Repeat("a", 64) + ".example.com"} {
		_, err := HostToASCII(input)
		assert.Error(t, err, input)
	}
}

func TestValidateName(t *testing.T) {
	for _, name := range []string{"a", "tm-prod-insure-ocr", strings.Repeat("a", 40)} {
		assert.NoError(t, ValidateName(name), name)
	}
	for _, name := range []string{"", "-a", "a-", "A", "a_b", strings.Repeat("a", 41)} {
		requireCode(t, ValidateName(name), ErrInvalidName)
	}
}
