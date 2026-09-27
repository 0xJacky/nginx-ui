package site

import (
	"errors"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// planFor plans config, builds the final configuration and re-parses both, so
// every assertion runs against output the site editor could load.
func planFor(t *testing.T, config string, req HTTPSPlanRequest) (plan *HTTPSPlan, staged, final *nginx.NgxConfig, stagedText, finalText string) {
	t.Helper()
	cfg := mustParseNgxConfig(t, config)
	before, err := cfg.BuildConfig()
	require.NoError(t, err)

	req.ChallengeLocation = testChallengeLocation
	plan, err = PlanHTTPS(cfg, req)
	require.NoError(t, err)
	finalCfg, err := plan.BuildFinal(testCertPath, testKeyPath)
	require.NoError(t, err)

	stagedText, staged = rebuild(t, plan.Staged)
	finalText, final = rebuild(t, finalCfg)

	after, err := cfg.BuildConfig()
	require.NoError(t, err)
	require.Equal(t, before, after, "planning must not modify its input")
	return plan, staged, final, stagedText, finalText
}

func requirePlanHint(t *testing.T, config string, req HTTPSPlanRequest, sentinel error, code string) *HTTPSHint {
	t.Helper()
	req.ChallengeLocation = testChallengeLocation
	plan, err := PlanHTTPS(mustParseNgxConfig(t, config), req)
	require.Error(t, err)
	assert.Nil(t, plan)
	assert.ErrorIs(t, err, sentinel)
	var hintErr *HTTPSHintError
	require.True(t, errors.As(err, &hintErr), "error carries no hint: %v", err)
	assert.Equal(t, code, hintErr.Hint.Code)
	assert.NotEmpty(t, hintErr.Hint.Message)
	return &hintErr.Hint
}

func diagnosticCodes(plan *HTTPSPlan) []string {
	codes := make([]string, 0, len(plan.Diagnostics))
	for _, diagnostic := range plan.Diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	return codes
}

func findDiagnostic(t *testing.T, plan *HTTPSPlan, code string) HTTPSDiagnostic {
	t.Helper()
	for _, diagnostic := range plan.Diagnostics {
		if diagnostic.Code == code {
			return diagnostic
		}
	}
	require.Failf(t, "diagnostic not found", "%s not in %v", code, diagnosticCodes(plan))
	return HTTPSDiagnostic{}
}

// --- Item 1: TLS servers whose certificate is not in the server block ---

const certificateFromHTTPBlockConfig = `
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
`

func TestPlanHTTPSKeepsLiveTLSServerWithCertificateFromHTTPBlock(t *testing.T) {
	// The zero value of the request is the enabled-site case: a live TLS
	// server is never dropped because it has no ssl_certificate of its own.
	for _, redirect := range []bool{true, false} {
		plan, staged, final, _, _ := planFor(t, certificateFromHTTPBlockConfig, HTTPSPlanRequest{
			Domains:             []string{"example.com"},
			ChallengeMethod:     HTTPSChallengeHTTP01,
			RedirectHTTPToHTTPS: redirect,
		})
		assert.True(t, plan.HasCertifiedTLS)
		assert.Zero(t, plan.PendingTLSServers)
		assert.False(t, plan.AddedHTTPServer)

		require.Len(t, staged.Servers, 2)
		stagedTLS := tlsServers(staged)
		require.Len(t, stagedTLS, 1)
		assert.Empty(t, directiveValues(stagedTLS[0], "ssl_certificate"), "the http{} certificate stays in charge until issued")
		assert.Contains(t, serverText(t, stagedTLS[0]), "proxy_pass http://127.0.0.1:9000")
		assert.Equal(t, 1, challengeCount(stagedTLS[0]))
		assertCanonicalRedirect(t, plainServers(staged)[0])

		require.Len(t, final.Servers, 2)
		tls := tlsServers(final)
		require.Len(t, tls, 1)
		assert.Equal(t, []string{"443 ssl"}, directiveValues(tls[0], "listen"))
		assert.Equal(t, []string{testCertPath}, directiveValues(tls[0], "ssl_certificate"))
		assert.Equal(t, []string{testKeyPath}, directiveValues(tls[0], "ssl_certificate_key"))
		assert.Contains(t, serverText(t, tls[0]), "proxy_pass http://127.0.0.1:9000")
		assertCanonicalRedirect(t, plainServers(final)[0])
	}
}

func TestPlanHTTPSTreatsTLSServerWithoutCertificateAsPendingOnlyWhenSiteDisabled(t *testing.T) {
	plan, staged, _, stagedText, _ := planFor(t, certificateFromHTTPBlockConfig, HTTPSPlanRequest{
		Domains:             []string{"example.com"},
		ChallengeMethod:     HTTPSChallengeHTTP01,
		RedirectHTTPToHTTPS: true,
		SiteDisabled:        true,
	})
	assert.False(t, plan.HasCertifiedTLS)
	assert.Equal(t, 1, plan.PendingTLSServers)
	require.Len(t, staged.Servers, 1)
	assert.Empty(t, tlsServers(staged))
	assert.Contains(t, stagedText, "proxy_pass http://127.0.0.1:9000")
}

func TestPlanHTTPSRefusesCertificateFromInclude(t *testing.T) {
	configs := map[string]string{
		"tls server": `
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
    include snippets/example-ssl.conf;
    location / {
        proxy_pass http://127.0.0.1:9000;
    }
}
`,
		"combined server with only the key": `
server {
    listen 80;
    listen 443 ssl;
    server_name example.com;
    ssl_certificate_key /etc/nginx/ssl/old/private.key;
    include snippets/example-ssl.conf;
    root /srv/app;
}
`,
	}
	for name, config := range configs {
		for _, disabled := range []bool{false, true} {
			t.Run(name, func(t *testing.T) {
				hint := requirePlanHint(t, config, HTTPSPlanRequest{
					Domains:         []string{"example.com"},
					ChallengeMethod: HTTPSChallengeHTTP01,
					SiteDisabled:    disabled,
				}, ErrHTTPSCertificateFromInclude, HTTPSHintCertificateFromInclude)
				assert.Equal(t, "example.com", hint.Params["server_name"])
				assert.Contains(t, hint.Message, "include")
			})
		}
	}
}

func TestPlanHTTPSAcceptsIncludeNextToExplicitCertificate(t *testing.T) {
	plan, _, final, _, finalText := planFor(t, `
server {
    listen 443 ssl;
    server_name example.com;
    ssl_certificate /etc/nginx/ssl/old/fullchain.cer;
    ssl_certificate_key /etc/nginx/ssl/old/private.key;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    root /srv/app;
}
`, HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, SiteDisabled: true})
	assert.True(t, plan.HasCertifiedTLS)
	tls := tlsServers(final)
	require.Len(t, tls, 1)
	assert.Equal(t, []string{testCertPath}, directiveValues(tls[0], "ssl_certificate"))
	assert.Contains(t, finalText, "include /etc/letsencrypt/options-ssl-nginx.conf;")
}

// --- Item 2: no server of the site serves the names ---

func TestPlanHTTPSRefusesDomainsNotInServerName(t *testing.T) {
	configs := []string{
		`
server {
    listen 80;
    server_name other.test;
    root /srv/other;
}
`,
		`
server {
    listen 80 default_server;
    server_name _;
    root /srv/app;
}
`,
	}
	for _, config := range configs {
		for _, method := range []string{HTTPSChallengeHTTP01, HTTPSChallengeDNS01} {
			hint := requirePlanHint(t, config, HTTPSPlanRequest{
				Domains:             []string{"example.com", "www.example.com"},
				ChallengeMethod:     method,
				RedirectHTTPToHTTPS: true,
			}, ErrHTTPSDomainsNotInServerName, HTTPSHintDomainsNotInServerName)
			assert.Equal(t, "example.com www.example.com", hint.Params["domains"])
			assert.Contains(t, hint.Message, "server_name")
		}
	}
}

func TestPlanHTTPSRefusesDomainsServedOnlyOnOtherPorts(t *testing.T) {
	config := `
server {
    listen 8080;
    server_name example.com;
    root /srv/app;
}
`
	for _, method := range []string{HTTPSChallengeHTTP01, HTTPSChallengeDNS01} {
		requirePlanHint(t, config, HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: method},
			ErrHTTPSDomainsNotOnStandardPorts, HTTPSHintDomainsNotOnStandardPorts)
	}
}

func TestPlanHTTPSAddedHTTPServerRedirectsToCertifiedTLS(t *testing.T) {
	for _, redirect := range []bool{false, true} {
		plan, staged, final, _, _ := planFor(t, `
server {
    listen 443 ssl;
    server_name example.com;
    ssl_certificate /etc/nginx/ssl/old/fullchain.cer;
    ssl_certificate_key /etc/nginx/ssl/old/private.key;
    root /srv/app;
}
`, HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: redirect})
		assert.True(t, plan.AddedHTTPServer)

		// Never a challenge-only server: ordinary requests reach the working
		// HTTPS server through the redirect.
		for _, cfg := range []*nginx.NgxConfig{staged, final} {
			plain := plainServers(cfg)
			require.Len(t, plain, 1)
			assertCanonicalRedirect(t, plain[0])
			assert.Equal(t, []string{"example.com"}, directiveValues(plain[0], "server_name"))
			tls := tlsServers(cfg)
			require.Len(t, tls, 1)
			assert.Equal(t, []string{"/srv/app"}, directiveValues(tls[0], "root"))
		}
		assert.Equal(t, []string{testCertPath}, directiveValues(tlsServers(final)[0], "ssl_certificate"))
	}
}

func TestPlanHTTPSAddedHTTPServerServesPendingApp(t *testing.T) {
	plan, staged, _, _, _ := planFor(t, `
server {
    listen 443 ssl;
    server_name example.com;
    location / {
        proxy_pass http://127.0.0.1:3000;
    }
}
`, HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: true, SiteDisabled: true})
	assert.True(t, plan.AddedHTTPServer)
	require.Len(t, staged.Servers, 1)
	content, ok := rootLocationContent(staged.Servers[0])
	require.True(t, ok, "the added server must serve the app, not only the challenge")
	assert.Contains(t, content, "proxy_pass http://127.0.0.1:3000")
}

// --- Item 3: a server listening on 80 and 443 in one block ---

const certifiedCombinedConfig = `
server {
    listen 80;
    listen 443 ssl;
    server_name example.com;
    ssl_certificate /etc/nginx/ssl/old/fullchain.cer;
    ssl_certificate_key /etc/nginx/ssl/old/private.key;
    if ($scheme != "https") {
        return 301 https://$host$request_uri;
    }
    location / {
        proxy_pass http://127.0.0.1:9000;
    }
}
`

func TestPlanHTTPSCertifiedCombinedServerKeepsSchemeRedirect(t *testing.T) {
	for _, redirect := range []bool{true, false} {
		plan, staged, final, stagedText, finalText := planFor(t, certifiedCombinedConfig, HTTPSPlanRequest{
			Domains:             []string{"example.com"},
			ChallengeMethod:     HTTPSChallengeHTTP01,
			RedirectHTTPToHTTPS: redirect,
		})
		assert.True(t, plan.HasCertifiedTLS)
		assert.False(t, plan.AddedHTTPServer, "a combined server is the port-80 server of its names")
		assert.Empty(t, plan.Diagnostics)

		for _, text := range []string{stagedText, finalText} {
			assert.Contains(t, text, `if ($scheme != "https"`)
			assert.Equal(t, 1, strings.Count(text, "return 301 https://$host$request_uri;"), text)
		}
		require.Len(t, staged.Servers, 1)
		assert.Equal(t, []string{"/etc/nginx/ssl/old/fullchain.cer"}, directiveValues(staged.Servers[0], "ssl_certificate"))
		assert.Equal(t, 1, challengeCount(staged.Servers[0]))

		require.Len(t, final.Servers, 1)
		server := final.Servers[0]
		assert.Equal(t, []string{"80", "443 ssl"}, directiveValues(server, "listen"))
		assert.Equal(t, []string{testCertPath}, directiveValues(server, "ssl_certificate"))
		assert.Equal(t, []string{testKeyPath}, directiveValues(server, "ssl_certificate_key"))
		assert.Equal(t, 1, challengeCount(server))
		assert.False(t, plan.StagedListensIPv6())
	}
}

func TestPlanHTTPSCertifiedCombinedServerWithoutRedirectIsNotRestructured(t *testing.T) {
	plan, _, final, _, finalText := planFor(t, `
server {
    listen 80;
    listen 443 ssl;
    server_name example.com;
    location / {
        proxy_pass http://127.0.0.1:9000;
    }
}
`, HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: true})
	// Enabled site: the certificate comes from the http{} block.
	assert.True(t, plan.HasCertifiedTLS)
	assert.False(t, plan.AddedHTTPServer)
	assert.Equal(t, []string{HTTPSDiagnosticRedirectNotAppliedCombinedServer}, diagnosticCodes(plan))

	require.Len(t, final.Servers, 1)
	assert.Equal(t, []string{"80", "443 ssl"}, directiveValues(final.Servers[0], "listen"))
	assert.Equal(t, []string{testCertPath}, directiveValues(final.Servers[0], "ssl_certificate"))
	assert.NotContains(t, finalText, "https://")
}

func TestPlanHTTPSSplitsPendingCombinedServerForRedirect(t *testing.T) {
	plan, staged, final, stagedText, _ := planFor(t, `
server {
    listen 80;
    listen [::]:80;
    listen 443 ssl;
    listen [::]:443 ssl;
    server_name example.com;
    root /srv/app;
    location / {
        try_files $uri /index.html;
    }
}
`, HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: true, SiteDisabled: true})
	assert.Equal(t, 1, plan.PendingTLSServers)
	assert.Empty(t, plan.Diagnostics)

	require.Len(t, staged.Servers, 1)
	assert.Equal(t, []string{"80", "[::]:80"}, directiveValues(staged.Servers[0], "listen"))
	assert.Contains(t, stagedText, "try_files $uri /index.html;")
	assert.NotContains(t, stagedText, "https://")

	require.Len(t, final.Servers, 2)
	plain := plainServers(final)
	require.Len(t, plain, 1)
	assertCanonicalRedirect(t, plain[0])
	assert.Equal(t, []string{"80", "[::]:80"}, directiveValues(plain[0], "listen"))
	assert.Empty(t, directiveValues(plain[0], "root"))

	tls := tlsServers(final)
	require.Len(t, tls, 1)
	assert.Equal(t, []string{"443 ssl", "[::]:443 ssl"}, directiveValues(tls[0], "listen"))
	assert.Equal(t, []string{testCertPath}, directiveValues(tls[0], "ssl_certificate"))
	assert.Equal(t, []string{"/srv/app"}, directiveValues(tls[0], "root"))
	assertNoHTTPSRedirect(t, tls[0])
}

// --- Item 4: every form of HTTPS redirect ---

func TestPlanHTTPSStripsEveryRedirectToTheSiteWhileTLSIsPending(t *testing.T) {
	redirects := map[string]string{
		"literal host return": `
    return 301 https://example.com$request_uri;`,
		"server-level rewrite": `
    rewrite ^ https://$host$request_uri permanent;`,
		"rewrite in location": `
    location / {
        rewrite ^/(.*)$ https://example.com/$1 redirect;
    }`,
		"quoted return 308 to server_name": `
    location / {
        return 308 "https://$server_name$request_uri";
    }`,
		"scheme guarded if block": `
    if ($scheme = http) {
        return 301 https://example.com$request_uri;
    }`,
	}
	for name, redirect := range redirects {
		t.Run(name, func(t *testing.T) {
			plan, staged, final, stagedText, _ := planFor(t, `
server {
    listen 80;
    server_name example.com;`+redirect+`
}
server {
    listen 443 ssl;
    server_name example.com;
    location / {
        proxy_pass http://127.0.0.1:3000;
    }
}
`, HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: true, SiteDisabled: true})
			assert.Empty(t, plan.Diagnostics)
			require.Len(t, staged.Servers, 1)
			assert.NotContains(t, stagedText, "https://", "a redirect to the pending HTTPS server survived")
			assert.NotContains(t, stagedText, "if (")
			content, ok := rootLocationContent(staged.Servers[0])
			require.True(t, ok)
			assert.Contains(t, content, "proxy_pass http://127.0.0.1:3000", "port 80 serves the app until issued")
			assert.Equal(t, 1, challengeCount(staged.Servers[0]))

			assertCanonicalRedirect(t, plainServers(final)[0])
			tls := tlsServers(final)
			require.Len(t, tls, 1)
			assertNoHTTPSRedirect(t, tls[0])
		})
	}
}

func TestPlanHTTPSFinalStripsOnlyLoopingRedirectsOnTLS(t *testing.T) {
	_, staged, final, _, _ := planFor(t, `
server {
    listen 80;
    server_name example.com www.example.com;
    location / {
        return 301 https://$host$request_uri;
    }
}
server {
    listen 443 ssl;
    server_name example.com www.example.com;
    if ($host != $server_name) {
        return 301 https://$server_name$request_uri;
    }
    rewrite ^ https://$host$request_uri permanent;
    location / {
        if ($http_x_forwarded_proto = http) {
            return 301 https://$host$request_uri;
        }
        proxy_pass http://127.0.0.1:9000;
    }
    location /old {
        return 301 https://$host/new;
    }
    location /self {
        return 301 https://www.example.com$request_uri;
    }
    location /external {
        return 302 https://docs.example.net$request_uri;
    }
}
`, HTTPSPlanRequest{Domains: []string{"example.com", "www.example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: true, SiteDisabled: true})

	// While TLS is pending every redirect to the site is gone from port 80.
	stagedPort80 := serverText(t, staged.Servers[0])
	assert.NotContains(t, stagedPort80, "https://$")
	assert.NotContains(t, stagedPort80, "https://www.example.com")
	assert.Contains(t, stagedPort80, "https://docs.example.net$request_uri")

	tls := tlsServers(final)
	require.Len(t, tls, 1)
	text := serverText(t, tls[0])
	// Guarded canonical-host and scheme redirects cannot loop and stay.
	assert.Contains(t, text, "if ($host != $server_name)")
	assert.Contains(t, text, "return 301 https://$server_name$request_uri;")
	assert.Contains(t, text, "if ($http_x_forwarded_proto = http)")
	// A path redirect on the same host and a redirect elsewhere are features.
	assert.Contains(t, text, "return 301 https://$host/new;")
	assert.Contains(t, text, "return 302 https://docs.example.net$request_uri;")
	// Unguarded redirects back to the same URL loop (F2) and are removed.
	assert.NotContains(t, text, "rewrite ^ https://$host$request_uri")
	assert.NotContains(t, text, "https://www.example.com$request_uri")
	assert.NotContains(t, text, "location /self")
	assert.Contains(t, text, "proxy_pass http://127.0.0.1:9000")
}

// --- Item 5: names the certificate does not cover ---

func TestPlanHTTPSLimitsTLSServerToCertificateNames(t *testing.T) {
	plan, _, final, _, _ := planFor(t, `
server {
    listen 80;
    server_name example.com www.example.com;
    location / {
        return 301 https://$host$request_uri;
    }
}
server {
    listen 443 ssl;
    server_name example.com www.example.com _ ~^static\d+\.example\.com$;
    location / {
        proxy_pass http://127.0.0.1:9000;
    }
}
`, HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: true, SiteDisabled: true})

	names := findDiagnostic(t, plan, HTTPSDiagnosticNamesNotInCertificate)
	assert.Equal(t, `www.example.com _ ~^static\d+\.example\.com$`, names.Params["names"])
	notApplied := findDiagnostic(t, plan, HTTPSDiagnosticRedirectNotAppliedUncoveredNames)
	assert.Equal(t, "www.example.com", notApplied.Params["names"])

	tls := tlsServers(final)
	require.Len(t, tls, 1)
	assert.Equal(t, []string{"example.com"}, directiveValues(tls[0], "server_name"))

	// The port-80 server keeps serving www.example.com instead of sending it
	// to a certificate that does not cover it.
	plain := plainServers(final)
	require.Len(t, plain, 1)
	assertNoHTTPSRedirect(t, plain[0])
	assert.Contains(t, serverText(t, plain[0]), "proxy_pass http://127.0.0.1:9000")
	assert.Equal(t, []string{"example.com www.example.com"}, directiveValues(plain[0], "server_name"))
}

func TestPlanHTTPSWildcardCertificateCoversServerNames(t *testing.T) {
	plan, _, final, _, _ := planFor(t, `
server {
    listen 80;
    server_name example.com *.example.com a.example.com;
    root /srv/app;
}
`, HTTPSPlanRequest{Domains: []string{"example.com", "*.example.com"}, ChallengeMethod: HTTPSChallengeDNS01, RedirectHTTPToHTTPS: true})
	assert.Empty(t, plan.Diagnostics)
	tls := tlsServers(final)
	require.Len(t, tls, 1)
	assert.Equal(t, []string{"example.com *.example.com a.example.com"}, directiveValues(tls[0], "server_name"))
	plain := plainServers(final)
	require.Len(t, plain, 1)
	content, ok := rootLocationContent(plain[0])
	require.True(t, ok)
	assert.True(t, contentRedirectsOnlyToHTTPS(content), content)
}

func TestPlanHTTPSRegexOnlyServerNameIsReplacedByTheDomains(t *testing.T) {
	plan, _, final, _, _ := planFor(t, `
server {
    listen 80;
    server_name ~^(www\.)?example\.com$;
    root /srv/app;
}
`, HTTPSPlanRequest{Domains: []string{"example.com", "www.example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: true})
	assert.ElementsMatch(t, []string{HTTPSDiagnosticNamesNotInCertificate, HTTPSDiagnosticRedirectNotAppliedUncoveredNames}, diagnosticCodes(plan))
	tls := tlsServers(final)
	require.Len(t, tls, 1)
	assert.Equal(t, []string{"example.com www.example.com"}, directiveValues(tls[0], "server_name"))
	plain := plainServers(final)
	require.Len(t, plain, 1)
	assert.Equal(t, []string{"/srv/app"}, directiveValues(plain[0], "root"))
}

// Review scenario I: two port-80 servers claim the same name.
func TestPlanHTTPSDuplicateServerNamesGenerateOneTLSServerPerName(t *testing.T) {
	plan, _, final, _, _ := planFor(t, `
server {
    listen 80;
    server_name example.com www.example.com;
    location / {
        proxy_pass http://127.0.0.1:3000;
    }
}
server {
    listen 80;
    server_name example.com;
    location /x {
        return 200;
    }
}
`, HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: true})

	tls := tlsServers(final)
	require.Len(t, tls, 1, "a second TLS server for example.com would be a conflicting server name")
	assert.Equal(t, []string{"example.com"}, directiveValues(tls[0], "server_name"))
	assert.Contains(t, serverText(t, tls[0]), "proxy_pass http://127.0.0.1:3000")

	// www.example.com is not in the certificate: its server keeps serving HTTP.
	notApplied := findDiagnostic(t, plan, HTTPSDiagnosticRedirectNotAppliedUncoveredNames)
	assert.Equal(t, "example.com www.example.com", notApplied.Params["server_name"])
	first := final.Servers[0]
	assert.Equal(t, []string{"example.com www.example.com"}, directiveValues(first, "server_name"))
	assertNoHTTPSRedirect(t, first)
}

// Review scenario J: a non-redirect server-level if and nested locations are
// application content and survive staging and the generated TLS server.
func TestPlanHTTPSKeepsServerLevelIfAndNestedLocations(t *testing.T) {
	_, staged, final, stagedText, _ := planFor(t, `
server {
    listen 80;
    server_name example.com;
    if ($http_user_agent ~* bot) {
        return 403;
    }
    location / {
        proxy_pass http://127.0.0.1:3000;
        location ~ \.php$ {
            deny all;
        }
    }
}
`, HTTPSPlanRequest{Domains: []string{"example.com"}, ChallengeMethod: HTTPSChallengeHTTP01, RedirectHTTPToHTTPS: true})
	require.Len(t, staged.Servers, 1)
	assert.Contains(t, stagedText, "if ($http_user_agent ~* bot)")
	assert.Contains(t, stagedText, "deny all;")

	tls := tlsServers(final)
	require.Len(t, tls, 1)
	text := serverText(t, tls[0])
	assert.Contains(t, text, "if ($http_user_agent ~* bot)")
	assert.Contains(t, text, "return 403;")
	assert.Contains(t, text, "deny all;")
	assertCanonicalRedirect(t, plainServers(final)[0])
}

func TestNameCoveredByCertificate(t *testing.T) {
	domains := []string{"example.com", "*.apps.example.com"}
	tests := []struct {
		name string
		want bool
	}{
		{"example.com", true},
		{"Example.COM.", true},
		{"www.example.com", false},
		{"a.apps.example.com", true},
		{"a.b.apps.example.com", false},
		{"apps.example.com", false},
		{"*.apps.example.com", true},
		{"*.example.com", false},
		{".apps.example.com", false},
		{"www.*", false},
		{"_", false},
		{"", false},
		{`~^example\.com$`, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, nameCoveredByCertificate(tt.name, domains), tt.name)
	}
	assert.True(t, nameCoveredByCertificate(".example.com", []string{"example.com", "*.example.com"}))
}

func TestHTTPSRedirectTarget(t *testing.T) {
	tests := []struct {
		statement, target, host string
		ok, keepsURI            bool
	}{
		{"return 301 https://$host$request_uri;", "https://$host$request_uri", "$host", true, true},
		{`return 308 "https://${host}${request_uri}"`, "https://${host}${request_uri}", "$host", true, true},
		{"return https://example.com$uri", "https://example.com$uri", "example.com", true, true},
		{"return 302 https://Example.com:8443/path", "https://Example.com:8443/path", "example.com", true, false},
		{"rewrite ^ https://$server_name$request_uri permanent", "https://$server_name$request_uri", "$server_name", true, true},
		{"rewrite ^/(.*)$ https://example.com/$1 last", "https://example.com/$1", "example.com", true, false},
		{"return 200 https://example.com", "", "", false, false},
		{"return 301 http://$host$request_uri", "", "", false, false},
		{"rewrite ^/old /new permanent", "", "", false, false},
		{"proxy_pass https://backend", "", "", false, false},
	}
	for _, tt := range tests {
		target, ok := httpsRedirectTarget(tt.statement)
		assert.Equal(t, tt.ok, ok, tt.statement)
		assert.Equal(t, tt.target, target, tt.statement)
		if ok {
			assert.Equal(t, tt.host, redirectTargetHost(target), tt.statement)
			assert.Equal(t, tt.keepsURI, keepsRequestURI(target), tt.statement)
		}
	}
}
