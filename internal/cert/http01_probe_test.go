package cert

import (
	"context"
	stderrors "errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
)

// fakeNginx stands in for the local Nginx: one plain HTTP server playing
// port 80 and one TLS server playing port 443, both routing by Host.
type fakeNginx struct {
	http  *httptest.Server
	https *httptest.Server
}

func (f *fakeNginx) endpoint() http01ProbeEndpoint {
	return http01ProbeEndpoint{HTTP: f.http.Listener.Addr().String(), HTTPS: f.https.Listener.Addr().String()}
}

// freeTCPPort returns a loopback port that was free a moment ago.
func freeTCPPort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	require.NoError(t, listener.Close())
	return port
}

// fakeHTTP01Blocks is a catch-all default server listening on the wildcard
// sockets of ports 80 and 443, the layout the fake Nginx plays by default.
func fakeHTTP01Blocks() []nginx.ServerBlock {
	return []nginx.ServerBlock{{
		File:        "/etc/nginx/conf.d/default.conf",
		ServerNames: []string{"_"},
		Listens: []nginx.ServerListen{
			{Port: "80", DefaultServer: true},
			{Port: "443", SSL: true, DefaultServer: true},
		},
	}}
}

// useHTTP01Blocks makes the probe discover blocks and dial every socket
// through addresses, keyed by serverListenLabel (e.g. "*:80",
// "192.0.2.10:80"). Sockets missing from addresses are not dialable.
func useHTTP01Blocks(blocks []nginx.ServerBlock, addresses map[string]string) {
	http01ProbeServerBlocks = func() ([]nginx.ServerBlock, error) { return blocks, nil }
	http01ProbeSocketAddress = func(listen nginx.ServerListen) string {
		return addresses[serverListenLabel(listen)]
	}
}

// setupHTTP01Probe points the probe at a free loopback challenge port and a
// fake Nginx whose wildcard sockets on ports 80 and 443 are served by the
// fake's HTTP and HTTPS servers. routes maps Host to a handler for plain
// HTTP, tlsRoutes for HTTPS; the special handler value nil means "proxy to
// the challenge port".
func setupHTTP01Probe(t *testing.T, routes, tlsRoutes map[string]http.Handler) (*fakeNginx, string) {
	t.Helper()
	port := freeTCPPort(t)

	previousHost := http01ProbeListenHost
	previousPort := http01ProbeChallengePort
	previousBlocks := http01ProbeServerBlocks
	previousSocket := http01ProbeSocketAddress
	previousEndpoints := http01ProbeDefaultEndpoints
	previousSite := http01ProbeSiteEndpoints
	previousSkip := http01ProbeSkipReason
	previousTimeout := http01ProbeRequestTimeout
	previousSettle := http01ProbeSettleTimeout
	previousInterval := http01ProbeSettleInterval
	t.Cleanup(func() {
		http01ProbeListenHost = previousHost
		http01ProbeChallengePort = previousPort
		http01ProbeServerBlocks = previousBlocks
		http01ProbeSocketAddress = previousSocket
		http01ProbeDefaultEndpoints = previousEndpoints
		http01ProbeSiteEndpoints = previousSite
		http01ProbeSkipReason = previousSkip
		http01ProbeRequestTimeout = previousTimeout
		http01ProbeSettleTimeout = previousSettle
		http01ProbeSettleInterval = previousInterval
	})

	challenge := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", port)})
	router := func(table map[string]http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			handler, ok := table[host]
			if !ok {
				http.NotFound(w, r)
				return
			}
			if handler == nil {
				challenge.ServeHTTP(w, r)
				return
			}
			handler.ServeHTTP(w, r)
		})
	}

	fake := &fakeNginx{
		http:  httptest.NewServer(router(routes)),
		https: httptest.NewTLSServer(router(tlsRoutes)),
	}
	t.Cleanup(fake.http.Close)
	t.Cleanup(fake.https.Close)

	http01ProbeListenHost = "127.0.0.1"
	http01ProbeChallengePort = func() string { return port }
	useHTTP01Blocks(fakeHTTP01Blocks(), map[string]string{
		"*:80":  fake.endpoint().HTTP,
		"*:443": fake.endpoint().HTTPS,
	})
	// The loopback defaults must not be needed while the configuration is
	// readable; point them at a closed port so a regression shows up.
	closed := net.JoinHostPort("127.0.0.1", freeTCPPort(t))
	http01ProbeDefaultEndpoints = func() []http01ProbeEndpoint {
		return []http01ProbeEndpoint{{HTTP: closed, HTTPS: closed}}
	}
	http01ProbeSiteEndpoints = func(string) []http01ProbeEndpoint { return nil }
	http01ProbeSkipReason = func() string { return "" }
	http01ProbeRequestTimeout = 2 * time.Second
	// Keep failing cases fast; the settle tests set their own window.
	http01ProbeSettleTimeout = 100 * time.Millisecond
	http01ProbeSettleInterval = 10 * time.Millisecond
	return fake, port
}

func redirectTo(format string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := fmt.Sprintf(format, r.URL.RequestURI())
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}

func staticBody(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	})
}

func probe(t *testing.T, domains ...string) []HTTP01ProbeResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	results, err := ProbeHTTP01Routes(ctx, domains)
	require.NoError(t, err)
	require.Len(t, results, len(domains))
	return results
}

func requireCosyCode(t *testing.T, err error, code int32) *cosy.Error {
	t.Helper()
	require.Error(t, err)
	var cosyErr *cosy.Error
	require.True(t, stderrors.As(err, &cosyErr), "expected cosy error, got %T", err)
	assert.Equal(t, code, cosyErr.Code)
	return cosyErr
}

func TestProbeHTTP01RoutesSuccess(t *testing.T) {
	fake, port := setupHTTP01Probe(t, map[string]http.Handler{"ok.example.com": nil}, nil)

	results := probe(t, "ok.example.com")

	assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status)
	assert.Equal(t, fake.endpoint().HTTP, results[0].Target)
	assert.Equal(t, http.StatusOK, results[0].StatusCode)
	assert.Empty(t, results[0].Error)
	assert.NoError(t, HTTP01RouteCheckError(results, ""))

	// The challenge port is released once the probe returns, so lego can bind it.
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
	require.NoError(t, err)
	require.NoError(t, listener.Close())
}

func TestProbeHTTP01RoutesNotFound(t *testing.T) {
	setupHTTP01Probe(t, map[string]http.Handler{"missing.example.com": http.NotFoundHandler()}, nil)

	results := probe(t, "missing.example.com")

	assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
	assert.Equal(t, http.StatusNotFound, results[0].StatusCode)
	assert.Contains(t, results[0].Error, "unexpected status 404")

	cosyErr := requireCosyCode(t, HTTP01RouteCheckError(results, ""), 50058)
	assert.Equal(t, "missing.example.com", cosyErr.Params[0])
	assert.Contains(t, cosyErr.Error(), "HTTP-01 challenge route check failed for missing.example.com")
}

func TestProbeHTTP01RoutesWrongBody(t *testing.T) {
	setupHTTP01Probe(t, map[string]http.Handler{"wrong.example.com": staticBody("hello from the app")}, nil)

	results := probe(t, "wrong.example.com")

	assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
	assert.Equal(t, http.StatusOK, results[0].StatusCode)
	assert.Contains(t, results[0].Error, "not the challenge token")
	requireCosyCode(t, HTTP01RouteCheckError(results, ""), 50058)
}

func TestProbeHTTP01RoutesFollowsRedirectToHTTPSThatServesToken(t *testing.T) {
	setupHTTP01Probe(t,
		map[string]http.Handler{"secure.example.com": redirectTo("https://secure.example.com%s")},
		map[string]http.Handler{"secure.example.com": nil},
	)

	results := probe(t, "secure.example.com")

	assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status, results[0].Error)
	require.Len(t, results[0].Redirects, 1)
	assert.True(t, strings.HasPrefix(results[0].Redirects[0], "https://secure.example.com/.well-known/acme-challenge/"))
	assert.NoError(t, HTTP01RouteCheckError(results, ""))
}

func TestProbeHTTP01RoutesRedirectToHTTPSThatReturns404(t *testing.T) {
	setupHTTP01Probe(t,
		map[string]http.Handler{"secure.example.com": redirectTo("https://secure.example.com%s")},
		map[string]http.Handler{"secure.example.com": http.NotFoundHandler()},
	)

	results := probe(t, "secure.example.com")

	assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
	assert.Equal(t, http.StatusNotFound, results[0].StatusCode)
	assert.Contains(t, results[0].Error, "unexpected status 404")
	assert.Contains(t, results[0].Error, "redirect chain: http://secure.example.com/.well-known/acme-challenge/")
	assert.Contains(t, results[0].Error, "-> https://secure.example.com/")
	requireCosyCode(t, HTTP01RouteCheckError(results, ""), 50058)
}

func TestProbeHTTP01RoutesRejectsRedirectsTheCARefuses(t *testing.T) {
	loop := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.URL.Query().Get("n"))
		http.Redirect(w, r, fmt.Sprintf("http://loop.example.com%s?n=%d", r.URL.Path, n+1), http.StatusFound)
	})
	var followed int
	counting := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		followed++
		w.WriteHeader(http.StatusTeapot)
	})
	setupHTTP01Probe(t, map[string]http.Handler{
		"badport.example.com": redirectTo("https://badport.example.com:8443%s"),
		"iphost.example.com":  redirectTo("http://192.0.2.1%s"),
		"ftp.example.com":     redirectTo("ftp://ftp.example.com%s"),
		"loop.example.com":    loop,
		"self.example.com":    redirectTo("http://self.example.com%s"),
		"target.example.com":  counting,
	}, map[string]http.Handler{"badport.example.com": counting})

	tests := []struct {
		domain  string
		wantErr string
	}{
		{"badport.example.com", "redirect to port 8443"},
		{"iphost.example.com", "redirect to IP address 192.0.2.1"},
		{"ftp.example.com", "only follows http and https"},
		{"loop.example.com", "too many redirects"},
		{"self.example.com", "redirect loop"},
	}
	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			results := probe(t, tt.domain)
			assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
			assert.Contains(t, results[0].Error, tt.wantErr)
			requireCosyCode(t, HTTP01RouteCheckError(results, ""), 50058)
		})
	}
	assert.Zero(t, followed, "a refused redirect must not be requested")

	results := probe(t, "loop.example.com")
	assert.Len(t, results[0].Redirects, http01ProbeMaxRedirects)
}

func TestProbeHTTP01RoutesRedirectToForeignHostIsWarning(t *testing.T) {
	setupHTTP01Probe(t, map[string]http.Handler{
		"moved.example.com": redirectTo("https://cdn.example.net%s"),
	}, nil)

	results := probe(t, "moved.example.com")

	assert.Equal(t, HTTP01ProbeStatusWarning, results[0].Status)
	assert.Contains(t, results[0].Error, "cdn.example.net")
	assert.Contains(t, results[0].Error, "cannot be verified locally")
	assert.NoError(t, HTTP01RouteCheckError(results, ""), "a warning must not block issuance")
	assert.Contains(t, SummarizeHTTP01ProbeResults(results), "cannot verify locally")
}

func TestProbeHTTP01RoutesChallengePortBusy(t *testing.T) {
	_, port := setupHTTP01Probe(t, map[string]http.Handler{"ok.example.com": nil}, nil)
	busy, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
	require.NoError(t, err)
	t.Cleanup(func() { _ = busy.Close() })

	results, err := ProbeHTTP01Routes(context.Background(), []string{"ok.example.com"})

	assert.Nil(t, results)
	cosyErr := requireCosyCode(t, err, 50059)
	assert.Equal(t, port, cosyErr.Params[0])
}

func TestProbeHTTP01RoutesMultipleDomainsOneFailing(t *testing.T) {
	setupHTTP01Probe(t, map[string]http.Handler{
		"ok.example.com":      nil,
		"missing.example.com": http.NotFoundHandler(),
	}, nil)

	results := probe(t, "ok.example.com", "missing.example.com")

	assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status)
	assert.Equal(t, HTTP01ProbeStatusFailure, results[1].Status)
	cosyErr := requireCosyCode(t, HTTP01RouteCheckError(results, ""), 50058)
	assert.Equal(t, "missing.example.com", cosyErr.Params[0])

	summary := SummarizeHTTP01ProbeResults(results)
	assert.Contains(t, summary, "ok.example.com: ok via")
	assert.Contains(t, summary, "missing.example.com: unexpected status 404")
}

// twoSocketBlocks serves domain from a block on a specific-address socket
// and from a block on the wildcard socket.
func twoSocketBlocks(domain string) []nginx.ServerBlock {
	return []nginx.ServerBlock{
		{
			File:        "/etc/nginx/sites-enabled/specific",
			ServerNames: []string{domain},
			Listens:     []nginx.ServerListen{{Addr: "192.0.2.10", Port: "80"}},
		},
		{
			File:        "/etc/nginx/sites-enabled/wildcard",
			ServerNames: []string{domain},
			Listens:     []nginx.ServerListen{{Port: "80"}, {Port: "443", SSL: true}},
		},
	}
}

func TestProbeHTTP01RoutesTriesNextEndpoint(t *testing.T) {
	fake, _ := setupHTTP01Probe(t, map[string]http.Handler{"ok.example.com": nil}, nil)
	closed := net.JoinHostPort("127.0.0.1", freeTCPPort(t))
	useHTTP01Blocks(twoSocketBlocks("ok.example.com"), map[string]string{
		"192.0.2.10:80": closed,
		"*:80":          fake.endpoint().HTTP,
	})

	results := probe(t, "ok.example.com")

	assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status)
	assert.Equal(t, fake.endpoint().HTTP, results[0].Target)
	require.Len(t, results[0].Attempts, 2)
	assert.Equal(t, closed, results[0].Attempts[0].Target)
	assert.Equal(t, HTTP01ProbeStatusWarning, results[0].Attempts[0].Status)
	assert.Contains(t, results[0].Attempts[0].Error, "cannot be verified locally")
}

func TestProbeHTTP01RoutesPrefersHTTPAnswerOverConnectionError(t *testing.T) {
	fake, _ := setupHTTP01Probe(t, map[string]http.Handler{"missing.example.com": http.NotFoundHandler()}, nil)
	closed := net.JoinHostPort("127.0.0.1", freeTCPPort(t))
	useHTTP01Blocks(twoSocketBlocks("missing.example.com"), map[string]string{
		"192.0.2.10:80": closed,
		"*:80":          fake.endpoint().HTTP,
	})

	results := probe(t, "missing.example.com")

	// The 404 is HTTP evidence and outranks the refused connection.
	assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
	assert.Equal(t, fake.endpoint().HTTP, results[0].Target)
	assert.Equal(t, http.StatusNotFound, results[0].StatusCode)
	require.Len(t, results[0].Attempts, 2)
	assert.Equal(t, HTTP01ProbeStatusWarning, results[0].Attempts[0].Status)
	requireCosyCode(t, HTTP01RouteCheckError(results, ""), 50058)
}

func TestProbeHTTP01RoutesSkippedWhenNginxIsNotLocal(t *testing.T) {
	setupHTTP01Probe(t, nil, nil)
	http01ProbeSkipReason = func() string { return "Nginx is not local to nginx-ui (control mode: external_container)" }

	results := probe(t, "a.example.com", "b.example.com")

	assert.True(t, HTTP01ProbeSkipped(results))
	for _, result := range results {
		assert.Equal(t, HTTP01ProbeStatusSkipped, result.Status)
		assert.Contains(t, result.SkipReason, "external_container")
	}
	assert.NoError(t, HTTP01RouteCheckError(results, ""))
	assert.Contains(t, SummarizeHTTP01ProbeResults(results), "skipped")
}

func TestProbeHTTP01RoutesHonorsContextWhileLocked(t *testing.T) {
	setupHTTP01Probe(t, nil, nil)
	mutex.Lock()
	defer mutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err := ProbeHTTP01Routes(ctx, []string{"ok.example.com"})

	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// challengeProxy forwards a request to the probe's challenge server, like a
// correctly configured Nginx location would.
func challengeProxy(port string) http.Handler {
	return httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", port)})
}

// scriptedRoute answers the n-th request (0-based) with 404 when notFound(n)
// is true and routes it to the challenge server otherwise, mimicking old and
// new Nginx workers answering side by side during a reload.
func scriptedRoute(port string, notFound func(n int) bool) (http.Handler, *atomic.Int32) {
	proxy := challengeProxy(port)
	var requests atomic.Int32
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(requests.Add(1)) - 1
		if notFound(n) {
			http.NotFound(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	}), &requests
}

func TestProbeHTTP01RoutesWaitsForReloadToSettle(t *testing.T) {
	fake, port := setupHTTP01Probe(t, nil, nil)
	http01ProbeSettleTimeout = 5 * time.Second
	http01ProbeSettleInterval = 20 * time.Millisecond
	// The first three requests hit the old configuration (default server 404).
	route, requests := scriptedRoute(port, func(n int) bool { return n < 3 })
	fake.http.Config.Handler = route

	results := probe(t, "fresh.example.com")

	assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status, results[0].Error)
	assert.Equal(t, http.StatusOK, results[0].StatusCode)
	assert.Empty(t, results[0].Error)
	// Three failing rounds, then the success and its confirmation.
	assert.Equal(t, 5, results[0].Rounds)
	assert.EqualValues(t, 5, requests.Load())
	assert.NoError(t, HTTP01RouteCheckError(results, ""))
}

func TestProbeHTTP01RoutesConfirmsSuccessDuringReload(t *testing.T) {
	fake, port := setupHTTP01Probe(t, nil, nil)
	http01ProbeSettleTimeout = 5 * time.Second
	http01ProbeSettleInterval = 20 * time.Millisecond
	// A new worker answers first, then an old one: one success is not enough.
	route, requests := scriptedRoute(port, func(n int) bool { return n == 0 || n == 2 })
	fake.http.Config.Handler = route

	results := probe(t, "fresh.example.com")

	assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status, results[0].Error)
	assert.Equal(t, 5, results[0].Rounds)
	assert.EqualValues(t, 5, requests.Load())
}

func TestProbeHTTP01RoutesFailsAfterSettleDeadline(t *testing.T) {
	setupHTTP01Probe(t, map[string]http.Handler{"missing.example.com": http.NotFoundHandler()}, nil)
	http01ProbeSettleTimeout = 150 * time.Millisecond
	http01ProbeSettleInterval = 20 * time.Millisecond

	start := time.Now()
	results := probe(t, "missing.example.com")
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, 150*time.Millisecond)
	assert.Less(t, elapsed, 5*time.Second)
	assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
	assert.Equal(t, http.StatusNotFound, results[0].StatusCode)
	assert.Contains(t, results[0].Error, "unexpected status 404")
	assert.Greater(t, results[0].Rounds, 1)
	requireCosyCode(t, HTTP01RouteCheckError(results, ""), 50058)
}

func TestProbeHTTP01RoutesReportsLastObservedResult(t *testing.T) {
	fake, _ := setupHTTP01Probe(t, nil, nil)
	http01ProbeSettleTimeout = 150 * time.Millisecond
	http01ProbeSettleInterval = 20 * time.Millisecond
	// Redirect first, then a plain 404 until the deadline.
	var requests atomic.Int32
	fake.http.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			http.Redirect(w, r, "http://192.0.2.1"+r.URL.RequestURI(), http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	results := probe(t, "flaky.example.com")

	assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
	assert.Equal(t, http.StatusNotFound, results[0].StatusCode)
	assert.NotContains(t, results[0].Error, "192.0.2.1")
}

func TestProbeHTTP01RoutesSettleTimeoutOption(t *testing.T) {
	setupHTTP01Probe(t, map[string]http.Handler{"missing.example.com": http.NotFoundHandler()}, nil)
	http01ProbeSettleTimeout = time.Minute
	http01ProbeSettleInterval = 20 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	results, err := ProbeHTTP01Routes(ctx, []string{"missing.example.com"}, WithHTTP01ProbeSettleTimeout(0))

	require.NoError(t, err)
	assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
	assert.Equal(t, 1, results[0].Rounds)
}

func TestProbeHTTP01RoutesDoesNotRetryWarningsOrFinalFailures(t *testing.T) {
	setupHTTP01Probe(t, map[string]http.Handler{
		"moved.example.com": redirectTo("https://cdn.example.net%s"),
	}, nil)
	http01ProbeSettleTimeout = time.Minute

	results := probe(t, "moved.example.com", "*.example.com")

	assert.Equal(t, HTTP01ProbeStatusWarning, results[0].Status)
	assert.Equal(t, 1, results[0].Rounds)
	assert.Equal(t, HTTP01ProbeStatusFailure, results[1].Status)
	assert.Equal(t, 1, results[1].Rounds)
}

func TestProbeHTTP01RoutesSettleStopsWithContext(t *testing.T) {
	setupHTTP01Probe(t, map[string]http.Handler{"missing.example.com": http.NotFoundHandler()}, nil)
	http01ProbeSettleTimeout = time.Minute
	http01ProbeSettleInterval = 20 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	results, err := ProbeHTTP01Routes(ctx, []string{"missing.example.com"})

	require.NoError(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
	// The last complete round is reported, not the aborted one.
	assert.Equal(t, http.StatusNotFound, results[0].StatusCode)
}

func TestProbeHTTP01RoutesWildcardFails(t *testing.T) {
	setupHTTP01Probe(t, nil, nil)

	results := probe(t, "*.example.com")

	assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
	assert.Contains(t, results[0].Error, "wildcard")
}

func TestHTTP01RouteCheckErrorAddsStaticExplanation(t *testing.T) {
	fake, _ := setupHTTP01Probe(t, map[string]http.Handler{"example.com": redirectTo("https://example.com%s")}, nil)
	useHTTP01Blocks([]nginx.ServerBlock{
		{
			File:        "/etc/nginx/sites-enabled/example.com",
			ServerNames: []string{"example.com"},
			Listens:     []nginx.ServerListen{{Port: "80"}},
			Return:      "301 https://$host$request_uri",
		},
		{
			File:        "/etc/nginx/sites-enabled/example.com",
			ServerNames: []string{"example.com"},
			Listens:     []nginx.ServerListen{{Port: "443", SSL: true}},
			Locations:   []nginx.ServerLocation{{Path: "/", ProxyPass: "http://127.0.0.1:3000"}},
		},
	}, map[string]string{
		"*:80":  fake.endpoint().HTTP,
		"*:443": fake.endpoint().HTTPS,
	})

	results := probe(t, "example.com")
	require.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)

	// The configuration name no longer matters for the explanation.
	err := HTTP01RouteCheckError(results, "unrelated-certificate-name")
	requireCosyCode(t, err, 50058)
	assert.Contains(t, err.Error(), "possible cause:")
	assert.Contains(t, err.Error(), "the server block for example.com on *:80 in /etc/nginx/sites-enabled/example.com redirects to HTTPS")
	assert.Contains(t, err.Error(), "no HTTPS server block for example.com")
	assert.NotContains(t, err.Error(), "not enabled")
}

func TestProbeHTTP01RoutesOneToManyCertificate(t *testing.T) {
	// A certificate named "shared-cert" covers three SANs served by two
	// blocks; api.example.com only listens on a specific address.
	fake, port := setupHTTP01Probe(t, map[string]http.Handler{
		"a.example.com": nil,
		"b.example.com": nil,
	}, nil)
	// Only the specific-address socket serves api.example.com by name; the
	// wildcard socket would answer it with the default server's 404.
	specific := httptest.NewServer(challengeProxy(port))
	t.Cleanup(specific.Close)
	var loopbackHits atomic.Int32
	loopback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loopbackHits.Add(1)
		http.NotFound(w, r)
	}))
	t.Cleanup(loopback.Close)

	useHTTP01Blocks([]nginx.ServerBlock{
		{
			File:        "/etc/nginx/conf.d/default.conf",
			ServerNames: []string{"_"},
			Listens:     []nginx.ServerListen{{Port: "80", DefaultServer: true}},
		},
		{
			File:        "/etc/nginx/sites-enabled/www",
			ServerNames: []string{"a.example.com", "b.example.com"},
			Listens:     []nginx.ServerListen{{Port: "80"}},
		},
		{
			File:        "/etc/nginx/sites-enabled/api",
			ServerNames: []string{"api.example.com"},
			Listens:     []nginx.ServerListen{{Addr: "192.0.2.10", Port: "80"}},
		},
	}, map[string]string{
		"*:80":          fake.endpoint().HTTP,
		"192.0.2.10:80": specific.Listener.Addr().String(),
	})
	http01ProbeDefaultEndpoints = func() []http01ProbeEndpoint {
		return []http01ProbeEndpoint{{HTTP: loopback.Listener.Addr().String()}}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	results, err := ProbeHTTP01Routes(ctx, []string{"a.example.com", "b.example.com", "api.example.com"},
		WithHTTP01ProbeConfigName("shared-cert"))
	require.NoError(t, err)

	for _, result := range results {
		assert.Equal(t, HTTP01ProbeStatusSuccess, result.Status, "%s: %s", result.Domain, result.Error)
	}
	assert.Equal(t, fake.endpoint().HTTP, results[0].Target)
	assert.Equal(t, fake.endpoint().HTTP, results[1].Target)
	// api.example.com passes through its specific-address socket; the
	// wildcard socket only reaches it through the unrelated default server,
	// so it does not decide and is not probed.
	assert.Equal(t, specific.Listener.Addr().String(), results[2].Target)
	require.Len(t, results[2].Attempts, 1)
	assert.Zero(t, loopbackHits.Load(), "loopback defaults must not be used when nginx -T is readable")
	assert.NoError(t, HTTP01RouteCheckError(results, "shared-cert"))
}

func TestProbeHTTP01RoutesNginxOnlyOnNonLoopbackAddress(t *testing.T) {
	fake, _ := setupHTTP01Probe(t, map[string]http.Handler{"ok.example.com": nil}, nil)
	useHTTP01Blocks([]nginx.ServerBlock{{
		File:        "/etc/nginx/sites-enabled/ok",
		ServerNames: []string{"ok.example.com"},
		Listens:     []nginx.ServerListen{{Addr: "203.0.113.7", Port: "80"}},
	}}, map[string]string{"203.0.113.7:80": fake.endpoint().HTTP})

	results := probe(t, "ok.example.com")

	assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status, results[0].Error)
	require.Len(t, results[0].Attempts, 1)
	assert.Equal(t, fake.endpoint().HTTP, results[0].Target)
}

func TestProbeHTTP01RoutesNoPort80SocketIsWarning(t *testing.T) {
	setupHTTP01Probe(t, nil, nil)
	useHTTP01Blocks([]nginx.ServerBlock{{
		ServerNames: []string{"tls.example.com"},
		Listens:     []nginx.ServerListen{{Port: "443", SSL: true}},
	}}, nil)
	http01ProbeSettleTimeout = time.Minute

	results := probe(t, "tls.example.com")

	assert.Equal(t, HTTP01ProbeStatusWarning, results[0].Status)
	assert.Equal(t, "no server block listens on port 80 for tls.example.com; the certificate authority may reach it through a load balancer or port mapping", results[0].Error)
	assert.Equal(t, 1, results[0].Rounds)
	assert.NoError(t, HTTP01RouteCheckError(results, ""))
}

func TestProbeHTTP01RoutesConnectionRefusedEverywhereIsWarning(t *testing.T) {
	setupHTTP01Probe(t, nil, nil)
	closedV4 := net.JoinHostPort("127.0.0.1", freeTCPPort(t))
	closedV6 := net.JoinHostPort("127.0.0.1", freeTCPPort(t))
	useHTTP01Blocks([]nginx.ServerBlock{{
		ServerNames: []string{"down.example.com"},
		Listens:     []nginx.ServerListen{{Port: "80"}, {Port: "80", IPv6: true}},
	}}, map[string]string{"*:80": closedV4, "[::]:80": closedV6})
	http01ProbeSettleTimeout = time.Minute

	results := probe(t, "down.example.com")

	assert.Equal(t, HTTP01ProbeStatusWarning, results[0].Status)
	assert.Contains(t, results[0].Error, "the local Nginx did not answer")
	assert.Contains(t, results[0].Error, "cannot be verified locally")
	require.Len(t, results[0].Attempts, 2)
	// Connection errors are not caused by a reload, so they are not retried.
	assert.Equal(t, 1, results[0].Rounds)
	assert.NoError(t, HTTP01RouteCheckError(results, ""))
}

func TestProbeHTTP01RoutesHTTPSHopWithoutSocketIsWarning(t *testing.T) {
	fake, _ := setupHTTP01Probe(t, map[string]http.Handler{"secure.example.com": redirectTo("https://secure.example.com%s")}, nil)
	useHTTP01Blocks([]nginx.ServerBlock{{
		ServerNames: []string{"secure.example.com"},
		Listens:     []nginx.ServerListen{{Port: "80"}},
		Return:      "301 https://$host$request_uri",
	}}, map[string]string{"*:80": fake.endpoint().HTTP})

	results := probe(t, "secure.example.com")

	assert.Equal(t, HTTP01ProbeStatusWarning, results[0].Status)
	assert.Contains(t, results[0].Error, "no server block listens on port 443 for secure.example.com")
	assert.Contains(t, results[0].Error, "redirect chain:")
	assert.NoError(t, HTTP01RouteCheckError(results, ""))
}

func TestProbeHTTP01RoutesTLSHandshakeErrorIsWarning(t *testing.T) {
	// A plain HTTP server on the HTTPS socket fails the TLS handshake.
	fake, _ := setupHTTP01Probe(t, map[string]http.Handler{"secure.example.com": redirectTo("https://secure.example.com%s")}, nil)
	useHTTP01Blocks(fakeHTTP01Blocks(), map[string]string{
		"*:80":  fake.endpoint().HTTP,
		"*:443": fake.endpoint().HTTP,
	})

	results := probe(t, "secure.example.com")

	assert.Equal(t, HTTP01ProbeStatusWarning, results[0].Status)
	assert.Contains(t, results[0].Error, "cannot be verified locally")
	assert.NoError(t, HTTP01RouteCheckError(results, ""))
}

func TestProbeHTTP01RoutesConfigUnreadable(t *testing.T) {
	t.Run("no address left is a warning", func(t *testing.T) {
		setupHTTP01Probe(t, map[string]http.Handler{"ok.example.com": nil}, nil)
		http01ProbeServerBlocks = func() ([]nginx.ServerBlock, error) { return nil, nginx.ErrNginxTOutputEmpty }
		http01ProbeDefaultEndpoints = func() []http01ProbeEndpoint { return nil }

		results := probe(t, "ok.example.com")

		assert.Equal(t, HTTP01ProbeStatusWarning, results[0].Status)
		assert.Equal(t, "the local Nginx configuration could not be read", results[0].Error)
		assert.NoError(t, HTTP01RouteCheckError(results, ""))
	})

	t.Run("loopback success still passes", func(t *testing.T) {
		fake, _ := setupHTTP01Probe(t, map[string]http.Handler{"ok.example.com": nil}, nil)
		http01ProbeServerBlocks = func() ([]nginx.ServerBlock, error) { return nil, nginx.ErrNginxTOutputEmpty }
		http01ProbeDefaultEndpoints = func() []http01ProbeEndpoint { return []http01ProbeEndpoint{fake.endpoint()} }

		results := probe(t, "ok.example.com")

		assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status, results[0].Error)
	})

	t.Run("loopback 404 is only a warning", func(t *testing.T) {
		fake, _ := setupHTTP01Probe(t, map[string]http.Handler{"missing.example.com": http.NotFoundHandler()}, nil)
		http01ProbeServerBlocks = func() ([]nginx.ServerBlock, error) { return nil, nginx.ErrNginxTOutputEmpty }
		http01ProbeDefaultEndpoints = func() []http01ProbeEndpoint { return []http01ProbeEndpoint{fake.endpoint()} }

		results := probe(t, "missing.example.com")

		assert.Equal(t, HTTP01ProbeStatusWarning, results[0].Status)
		assert.Equal(t, http.StatusNotFound, results[0].StatusCode)
		assert.Contains(t, results[0].Error, "the local Nginx configuration could not be read")
		assert.NoError(t, HTTP01RouteCheckError(results, ""))
	})

	t.Run("loopback refused is a warning", func(t *testing.T) {
		setupHTTP01Probe(t, nil, nil)
		http01ProbeServerBlocks = func() ([]nginx.ServerBlock, error) { return nil, nginx.ErrNginxTOutputEmpty }

		results := probe(t, "ok.example.com")

		assert.Equal(t, HTTP01ProbeStatusWarning, results[0].Status)
		assert.Contains(t, results[0].Error, "the local Nginx configuration could not be read")
	})

	t.Run("site listen address is evidence", func(t *testing.T) {
		fake, _ := setupHTTP01Probe(t, map[string]http.Handler{"missing.example.com": http.NotFoundHandler()}, nil)
		http01ProbeServerBlocks = func() ([]nginx.ServerBlock, error) { return nil, nginx.ErrNginxTOutputEmpty }
		http01ProbeSiteEndpoints = func(name string) []http01ProbeEndpoint {
			assert.Equal(t, "example.com", name)
			return []http01ProbeEndpoint{fake.endpoint()}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		results, err := ProbeHTTP01Routes(ctx, []string{"missing.example.com"}, WithHTTP01ProbeConfigName("example.com"))
		require.NoError(t, err)

		assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
		assert.Equal(t, fake.endpoint().HTTP, results[0].Target)
		requireCosyCode(t, HTTP01RouteCheckError(results, "example.com"), 50058)
	})
}

func TestVerifyHTTP01ChallengeRouteNeverBlocksOnRouteResults(t *testing.T) {
	t.Run("HTTP failure is logged and issuance continues", func(t *testing.T) {
		setupHTTP01Probe(t, map[string]http.Handler{
			"ok.example.com":      nil,
			"missing.example.com": http.NotFoundHandler(),
		}, nil)
		log := NewLogger()
		defer log.Close()

		err := verifyHTTP01ChallengeRoute(&ConfigPayload{ServerName: []string{"ok.example.com", "missing.example.com"}, ConfigName: "none"}, log)

		assert.NoError(t, err)
		assert.Contains(t, log.ToString(), "HTTP01 challenge route reachable")
		assert.Contains(t, log.ToString(), "HTTP01 challenge route check failed for %{domain}: %{error}. Issuance continues; the certificate authority will verify the route itself")
		assert.Contains(t, log.ToString(), "missing.example.com")

		// The HTTPS card orchestrator still gets the 50058 error for the
		// same HTTP evidence.
		results := probe(t, "ok.example.com", "missing.example.com")
		requireCosyCode(t, HTTP01RouteCheckError(results, ""), 50058)
	})

	t.Run("connection refused everywhere is a warning", func(t *testing.T) {
		setupHTTP01Probe(t, nil, nil)
		closed := net.JoinHostPort("127.0.0.1", freeTCPPort(t))
		useHTTP01Blocks(fakeHTTP01Blocks(), map[string]string{"*:80": closed, "*:443": closed})
		log := NewLogger()
		defer log.Close()

		err := verifyHTTP01ChallengeRoute(&ConfigPayload{ServerName: []string{"down.example.com"}, ConfigName: "down.example.com"}, log)

		assert.NoError(t, err)
		assert.Contains(t, log.ToString(), "HTTP01 challenge route cannot be verified locally for %{domain}: %{reason}")
		assert.NotContains(t, log.ToString(), "check failed")
	})

	t.Run("unreadable configuration is a warning", func(t *testing.T) {
		setupHTTP01Probe(t, nil, nil)
		http01ProbeServerBlocks = func() ([]nginx.ServerBlock, error) { return nil, nginx.ErrNginxTOutputEmpty }
		log := NewLogger()
		defer log.Close()

		err := verifyHTTP01ChallengeRoute(&ConfigPayload{ServerName: []string{"ok.example.com"}, ConfigName: "cert-name"}, log)

		assert.NoError(t, err)
		assert.Contains(t, log.ToString(), "cannot be verified locally")
		assert.Contains(t, log.ToString(), "the local Nginx configuration could not be read")
	})

	t.Run("busy challenge port still blocks", func(t *testing.T) {
		_, port := setupHTTP01Probe(t, map[string]http.Handler{"ok.example.com": nil}, nil)
		busy, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
		require.NoError(t, err)
		t.Cleanup(func() { _ = busy.Close() })
		log := NewLogger()
		defer log.Close()

		err = verifyHTTP01ChallengeRoute(&ConfigPayload{ServerName: []string{"ok.example.com"}, ConfigName: "ok"}, log)

		requireCosyCode(t, err, 50059)
	})
}

func TestHTTP01SocketDialAddress(t *testing.T) {
	tests := []struct {
		listen nginx.ServerListen
		want   string
	}{
		{nginx.ServerListen{Port: "80"}, "127.0.0.1:80"},
		{nginx.ServerListen{Addr: "*", Port: "80"}, "127.0.0.1:80"},
		{nginx.ServerListen{Addr: "0.0.0.0", Port: "80"}, "127.0.0.1:80"},
		{nginx.ServerListen{Port: "80", IPv6: true}, "[::1]:80"},
		{nginx.ServerListen{Addr: "::", Port: "80", IPv6: true}, "[::1]:80"},
		{nginx.ServerListen{Addr: "192.0.2.10", Port: "80"}, "192.0.2.10:80"},
		{nginx.ServerListen{Addr: "2001:db8::1", Port: "80", IPv6: true}, "[2001:db8::1]:80"},
		{nginx.ServerListen{Addr: "[2001:db8::1]", Port: "443", IPv6: true, SSL: true}, "[2001:db8::1]:443"},
		{nginx.ServerListen{Addr: "192.0.2.10"}, "192.0.2.10:80"},
		{nginx.ServerListen{Addr: "unix:/run/nginx.sock"}, ""},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%+v", tt.listen), func(t *testing.T) {
			assert.Equal(t, tt.want, http01SocketDialAddress(tt.listen))
		})
	}
}

func TestHTTPListenTarget(t *testing.T) {
	tests := []struct {
		params string
		want   string
	}{
		{"80", ""},
		{"80 default_server", ""},
		{"[::]:80", ""},
		{"0.0.0.0:80", ""},
		{"*:80", ""},
		{"192.0.2.10:80", "192.0.2.10:80"},
		{"192.0.2.10", "192.0.2.10:80"},
		{"[2001:db8::1]:80 ipv6only=on", "[2001:db8::1]:80"},
		{"[2001:db8::1]", "[2001:db8::1]:80"},
		{"192.0.2.10:443 ssl", ""},
		{"8080", ""},
		{"unix:/run/nginx.sock", ""},
		{"localhost:80", "localhost:80"},
	}
	for _, tt := range tests {
		t.Run(tt.params, func(t *testing.T) {
			assert.Equal(t, tt.want, httpListenTarget(tt.params))
		})
	}
}

func TestHTTP01ProbeFallbackEndpointsPutSiteAddressesFirst(t *testing.T) {
	previousSite := http01ProbeSiteEndpoints
	previousDefaults := http01ProbeDefaultEndpoints
	t.Cleanup(func() {
		http01ProbeSiteEndpoints = previousSite
		http01ProbeDefaultEndpoints = previousDefaults
	})
	http01ProbeSiteEndpoints = func(name string) []http01ProbeEndpoint {
		assert.Equal(t, "example.com", name)
		return []http01ProbeEndpoint{{HTTP: "192.0.2.10:80", HTTPS: "192.0.2.10:443"}, {HTTP: "127.0.0.1:80", HTTPS: "127.0.0.1:443"}}
	}
	http01ProbeDefaultEndpoints = func() []http01ProbeEndpoint {
		return []http01ProbeEndpoint{{HTTP: "127.0.0.1:80", HTTPS: "127.0.0.1:443"}, {HTTP: "[::1]:80", HTTPS: "[::1]:443"}}
	}

	cfg := &http01ProbeConfig{}
	WithHTTP01ProbeConfigName("example.com")(cfg)
	endpoints := http01ProbeEndpoints(cfg)

	require.Len(t, endpoints, 3)
	assert.Equal(t, "192.0.2.10:80", endpoints[0].HTTP)
	assert.Equal(t, "127.0.0.1:80", endpoints[1].HTTP)
	assert.Equal(t, "[::1]:80", endpoints[2].HTTP)
	// Only the loopback defaults are guesses; the site's own listen address
	// can still produce HTTP evidence.
	assert.False(t, endpoints[0].guessed)
	assert.False(t, endpoints[1].guessed, "a site address wins over the identical default")
	assert.True(t, endpoints[2].guessed)
}

// serveHTTP01Socket starts a fake Nginx socket with handler and returns its
// address.
func serveHTTP01Socket(t *testing.T, handler http.Handler, useTLS bool) string {
	t.Helper()
	var server *httptest.Server
	if useTLS {
		server = httptest.NewTLSServer(handler)
	} else {
		server = httptest.NewServer(handler)
	}
	t.Cleanup(server.Close)
	return server.Listener.Addr().String()
}

// unrelatedIPv6DefaultBlocks is the E2E layout: d.example.com is only served
// by an IPv4 wildcard block, and another site owns the IPv6 default server.
func unrelatedIPv6DefaultBlocks(port string) []nginx.ServerBlock {
	return []nginx.ServerBlock{
		{
			File:        "/etc/nginx/sites-enabled/d",
			ServerNames: []string{"d.example.com"},
			Listens:     []nginx.ServerListen{{Port: "80"}},
		},
		{
			File:        "/etc/nginx/sites-enabled/other",
			ServerNames: []string{"other.example.com"},
			Listens:     []nginx.ServerListen{{Port: "80", IPv6: true, DefaultServer: true}},
			Locations:   []nginx.ServerLocation{http01ChallengeLocation("http://127.0.0.1:" + port)},
		},
	}
}

func TestProbeHTTP01RoutesIgnoresUnrelatedIPv6DefaultServer(t *testing.T) {
	t.Run("name-matched 404 is not hidden by a default-server success", func(t *testing.T) {
		_, port := setupHTTP01Probe(t, nil, nil)
		v4 := serveHTTP01Socket(t, http.NotFoundHandler(), false)
		v6 := serveHTTP01Socket(t, challengeProxy(port), false)
		useHTTP01Blocks(unrelatedIPv6DefaultBlocks(port), map[string]string{"*:80": v4, "[::]:80": v6})

		results := probe(t, "d.example.com")

		assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status, results[0].Error)
		assert.Equal(t, v4, results[0].Target)
		require.Len(t, results[0].Attempts, 1, "the IPv6 default server does not decide for d.example.com")
		err := HTTP01RouteCheckError(results, "")
		requireCosyCode(t, err, 50058)
		assert.Contains(t, err.Error(), "possible cause: the server block for d.example.com on *:80 in /etc/nginx/sites-enabled/d has no /.well-known/acme-challenge location")
		assert.NotContains(t, err.Error(), "sites-enabled/other")
	})

	t.Run("name-matched success is not failed by a default-server 404", func(t *testing.T) {
		_, port := setupHTTP01Probe(t, nil, nil)
		v4 := serveHTTP01Socket(t, challengeProxy(port), false)
		v6 := serveHTTP01Socket(t, http.NotFoundHandler(), false)
		useHTTP01Blocks(unrelatedIPv6DefaultBlocks(port), map[string]string{"*:80": v4, "[::]:80": v6})

		results := probe(t, "d.example.com")

		assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status, results[0].Error)
		assert.Equal(t, v4, results[0].Target)
		assert.NoError(t, HTTP01RouteCheckError(results, ""))
	})

	t.Run("default servers decide when no socket matches by name", func(t *testing.T) {
		_, port := setupHTTP01Probe(t, nil, nil)
		v4 := serveHTTP01Socket(t, http.NotFoundHandler(), false)
		v6 := serveHTTP01Socket(t, challengeProxy(port), false)
		useHTTP01Blocks(unrelatedIPv6DefaultBlocks(port), map[string]string{"*:80": v4, "[::]:80": v6})

		results := probe(t, "unknown.example.com")

		// Both sockets answer the name with their default server; the CA
		// may reach either, so the 404 decides.
		assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
		assert.Len(t, results[0].Attempts, 2)
	})
}

func TestProbeHTTP01RoutesAnyNameMatchedFailureFails(t *testing.T) {
	_, port := setupHTTP01Probe(t, nil, nil)
	ok := serveHTTP01Socket(t, challengeProxy(port), false)
	missing := serveHTTP01Socket(t, http.NotFoundHandler(), false)
	useHTTP01Blocks([]nginx.ServerBlock{{
		File:        "/etc/nginx/sites-enabled/d",
		ServerNames: []string{"d.example.com"},
		Listens:     []nginx.ServerListen{{Port: "80"}, {Port: "80", IPv6: true}},
	}}, map[string]string{"*:80": ok, "[::]:80": missing})

	results := probe(t, "d.example.com")

	// The CA may use either address family, so the 404 on one socket fails
	// the domain although the other one serves the token.
	assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
	assert.Equal(t, missing, results[0].Target)
	require.Len(t, results[0].Attempts, 2)
	assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Attempts[0].Status)
	requireCosyCode(t, HTTP01RouteCheckError(results, ""), 50058)
}

func TestProbeHTTP01RoutesCauseNamesFailingSocket(t *testing.T) {
	// The first socket in config order is statically broken too, but it
	// does not answer; the cause must describe the socket that returned 404.
	_, port := setupHTTP01Probe(t, nil, nil)
	closed := net.JoinHostPort("127.0.0.1", freeTCPPort(t))
	wrongPort := serveHTTP01Socket(t, http.NotFoundHandler(), false)
	useHTTP01Blocks([]nginx.ServerBlock{
		{
			File:        "/etc/nginx/sites-enabled/d-public",
			ServerNames: []string{"d.example.com"},
			Listens:     []nginx.ServerListen{{Addr: "10.31.0.10", Port: "80"}},
		},
		{
			File:        "/etc/nginx/sites-enabled/d",
			ServerNames: []string{"d.example.com"},
			Listens:     []nginx.ServerListen{{Port: "80"}},
			Locations:   []nginx.ServerLocation{http01ChallengeLocation("http://127.0.0.1:1")},
		},
	}, map[string]string{"10.31.0.10:80": closed, "*:80": wrongPort})

	results := probe(t, "d.example.com")

	require.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
	assert.Equal(t, wrongPort, results[0].Target)
	err := HTTP01RouteCheckError(results, "")
	requireCosyCode(t, err, 50058)
	assert.Contains(t, err.Error(), "possible cause: the server block for d.example.com on *:80 in /etc/nginx/sites-enabled/d has a /.well-known/acme-challenge location that does not proxy to 127.0.0.1:"+port)
	assert.NotContains(t, err.Error(), "10.31.0.10")
}

func TestProbeHTTP01RoutesRedirectHopUsesDecidingSockets(t *testing.T) {
	redirect := redirectTo("https://d.example.com%s")
	hopBlocks := func(port string) []nginx.ServerBlock {
		return []nginx.ServerBlock{
			{
				File:        "/etc/nginx/sites-enabled/d",
				ServerNames: []string{"d.example.com"},
				Listens:     []nginx.ServerListen{{Port: "80"}, {Port: "443", SSL: true}},
				Return:      "301 https://$host$request_uri",
			},
			{
				File:        "/etc/nginx/sites-enabled/other",
				ServerNames: []string{"other.example.com"},
				Listens:     []nginx.ServerListen{{Port: "443", IPv6: true, SSL: true, DefaultServer: true}},
				Locations:   []nginx.ServerLocation{http01ChallengeLocation("http://127.0.0.1:" + port)},
			},
		}
	}

	t.Run("name-matched 404 on the HTTPS hop fails", func(t *testing.T) {
		_, port := setupHTTP01Probe(t, nil, nil)
		http80 := serveHTTP01Socket(t, redirect, false)
		named := serveHTTP01Socket(t, http.NotFoundHandler(), true)
		other := serveHTTP01Socket(t, challengeProxy(port), true)
		useHTTP01Blocks(hopBlocks(port), map[string]string{"*:80": http80, "*:443": named, "[::]:443": other})

		results := probe(t, "d.example.com")

		assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status, results[0].Error)
		assert.Equal(t, http.StatusNotFound, results[0].StatusCode)
		err := HTTP01RouteCheckError(results, "")
		assert.Contains(t, err.Error(), "the server block for d.example.com on *:80 in /etc/nginx/sites-enabled/d redirects to HTTPS")
	})

	t.Run("any name-matched HTTPS socket failing fails", func(t *testing.T) {
		_, port := setupHTTP01Probe(t, nil, nil)
		http80 := serveHTTP01Socket(t, redirect, false)
		good := serveHTTP01Socket(t, challengeProxy(port), true)
		bad := serveHTTP01Socket(t, http.NotFoundHandler(), true)
		blocks := hopBlocks(port)
		blocks[0].Listens = append(blocks[0].Listens, nginx.ServerListen{Port: "443", IPv6: true, SSL: true})
		useHTTP01Blocks(blocks, map[string]string{"*:80": http80, "*:443": good, "[::]:443": bad})

		results := probe(t, "d.example.com")

		assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status, results[0].Error)
		assert.Equal(t, http.StatusNotFound, results[0].StatusCode)
	})

	t.Run("name-matched HTTPS success is not failed by a default server", func(t *testing.T) {
		_, port := setupHTTP01Probe(t, nil, nil)
		http80 := serveHTTP01Socket(t, redirect, false)
		named := serveHTTP01Socket(t, challengeProxy(port), true)
		other := serveHTTP01Socket(t, http.NotFoundHandler(), true)
		useHTTP01Blocks(hopBlocks(port), map[string]string{"*:80": http80, "*:443": named, "[::]:443": other})

		results := probe(t, "d.example.com")

		assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status, results[0].Error)
	})
}
