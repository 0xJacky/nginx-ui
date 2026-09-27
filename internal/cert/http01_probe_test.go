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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
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

// setupHTTP01Probe points the probe at a free loopback challenge port and a
// fake Nginx. routes maps Host to a handler for plain HTTP, tlsRoutes for
// HTTPS; the special handler value nil means "proxy to the challenge port".
func setupHTTP01Probe(t *testing.T, routes, tlsRoutes map[string]http.Handler) (*fakeNginx, string) {
	t.Helper()
	port := freeTCPPort(t)

	previousHost := http01ProbeListenHost
	previousPort := http01ProbeChallengePort
	previousEndpoints := http01ProbeDefaultEndpoints
	previousSite := http01ProbeSiteEndpoints
	previousSkip := http01ProbeSkipReason
	previousTimeout := http01ProbeRequestTimeout
	previousSettle := http01ProbeSettleTimeout
	previousInterval := http01ProbeSettleInterval
	t.Cleanup(func() {
		http01ProbeListenHost = previousHost
		http01ProbeChallengePort = previousPort
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
	http01ProbeDefaultEndpoints = func() []http01ProbeEndpoint { return []http01ProbeEndpoint{fake.endpoint()} }
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

func TestProbeHTTP01RoutesTriesNextEndpoint(t *testing.T) {
	fake, _ := setupHTTP01Probe(t, map[string]http.Handler{"ok.example.com": nil}, nil)
	closed := net.JoinHostPort("127.0.0.1", freeTCPPort(t))
	http01ProbeDefaultEndpoints = func() []http01ProbeEndpoint {
		return []http01ProbeEndpoint{{HTTP: closed, HTTPS: closed}, fake.endpoint()}
	}

	results := probe(t, "ok.example.com")

	assert.Equal(t, HTTP01ProbeStatusSuccess, results[0].Status)
	assert.Equal(t, fake.endpoint().HTTP, results[0].Target)
	require.Len(t, results[0].Attempts, 2)
	assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Attempts[0].Status)
	assert.NotEmpty(t, results[0].Attempts[0].Error)
}

func TestProbeHTTP01RoutesPrefersHTTPAnswerOverConnectionError(t *testing.T) {
	fake, _ := setupHTTP01Probe(t, map[string]http.Handler{"missing.example.com": http.NotFoundHandler()}, nil)
	closed := net.JoinHostPort("127.0.0.1", freeTCPPort(t))
	http01ProbeDefaultEndpoints = func() []http01ProbeEndpoint {
		return []http01ProbeEndpoint{{HTTP: closed, HTTPS: closed}, fake.endpoint()}
	}

	results := probe(t, "missing.example.com")

	assert.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)
	assert.Equal(t, fake.endpoint().HTTP, results[0].Target)
	assert.Equal(t, http.StatusNotFound, results[0].StatusCode)
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
	setupHTTP01Probe(t, map[string]http.Handler{"example.com": redirectTo("https://example.com%s")}, nil)

	configDir := t.TempDir()
	previousConfigDir := settings.NginxSettings.ConfigDir
	previousChallengePort := settings.CertSettings.HTTPChallengePort
	settings.NginxSettings.ConfigDir = configDir
	settings.CertSettings.HTTPChallengePort = "9180"
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = previousConfigDir
		settings.CertSettings.HTTPChallengePort = previousChallengePort
	})
	availableDir := filepath.Join(configDir, "sites-available")
	enabledDir := filepath.Join(configDir, "sites-enabled")
	require.NoError(t, os.MkdirAll(availableDir, 0755))
	require.NoError(t, os.MkdirAll(enabledDir, 0755))
	availablePath := filepath.Join(availableDir, "example.com")
	require.NoError(t, os.WriteFile(availablePath, []byte(`server {
	listen 80;
	server_name example.com;
	return 301 https://$host$request_uri;
}`), 0644))
	require.NoError(t, os.Symlink(availablePath, filepath.Join(enabledDir, "example.com")))

	results := probe(t, "example.com")
	require.Equal(t, HTTP01ProbeStatusFailure, results[0].Status)

	err := HTTP01RouteCheckError(results, "example.com")
	requireCosyCode(t, err, 50058)
	assert.Contains(t, err.Error(), "possible cause:")
	assert.Contains(t, err.Error(), "no HTTPS server for this name")
}

func TestVerifyHTTP01ChallengeRouteBlocksIssuanceOnFailure(t *testing.T) {
	setupHTTP01Probe(t, map[string]http.Handler{
		"ok.example.com":      nil,
		"missing.example.com": http.NotFoundHandler(),
	}, nil)
	log := NewLogger()
	defer log.Close()

	err := verifyHTTP01ChallengeRoute(&ConfigPayload{ServerName: []string{"ok.example.com"}, ConfigName: "none"}, log)
	assert.NoError(t, err)

	err = verifyHTTP01ChallengeRoute(&ConfigPayload{ServerName: []string{"ok.example.com", "missing.example.com"}, ConfigName: "none"}, log)
	requireCosyCode(t, err, 50058)
	assert.Contains(t, log.ToString(), "HTTP01 challenge route reachable")
	assert.Contains(t, log.ToString(), "HTTP01 challenge route check failed")
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

func TestHTTP01ProbeEndpointsPutsSiteAddressesFirst(t *testing.T) {
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
}
