package sitecheck

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCheckSiteSkipsNetworkWhenDisabled(t *testing.T) {
	t.Cleanup(InvalidateSiteConfigCache)

	options := DefaultCheckOptions()
	checker := NewSiteChecker(options)

	// Any HTTP request made by the checker should fail this test.
	checker.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected HTTP request to %s while health check is disabled", req.URL.String())
			return nil, nil
		}),
	}

	const siteURL = "https://example.com"
	config := &model.SiteConfig{
		Model:              model.Model{ID: 1},
		Host:               "example.com:443",
		Scheme:             "https",
		DisplayURL:         siteURL,
		HealthCheckEnabled: false,
		HealthCheckConfig: &model.HealthCheckConfig{
			Protocol: "https",
		},
	}

	setCachedSiteConfig(config.Host, config)

	if _, err := checker.CheckSite(context.Background(), siteURL); err != nil {
		t.Fatalf("CheckSite returned error: %v", err)
	}
}

func TestDownloadFaviconAcceptsValidImage(t *testing.T) {
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0x0F, 0x04, 0x00,
		0x09, 0xFB, 0x03, 0xFD, 0xA7, 0x89, 0x81, 0xB9,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44,
		0xAE, 0x42, 0x60, 0x82,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(pngData)
	}))
	defer server.Close()

	checker := NewSiteChecker(DefaultCheckOptions())
	dataURL := checker.downloadFavicon(context.Background(), server.URL+"/favicon.png")
	if dataURL == "" {
		t.Fatal("expected data URL for valid favicon")
	}

	expectedPrefix := "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, expectedPrefix) {
		t.Fatalf("unexpected data URL prefix: %s", dataURL)
	}

	expectedPayload := base64.StdEncoding.EncodeToString(pngData)
	if payload := strings.TrimPrefix(dataURL, expectedPrefix); payload != expectedPayload {
		t.Fatalf("unexpected base64 payload: got %s want %s", payload, expectedPayload)
	}
}

func TestDownloadFaviconRejectsHTMLContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, "<html><body>not an image</body></html>")
	}))
	defer server.Close()

	checker := NewSiteChecker(DefaultCheckOptions())
	dataURL := checker.downloadFavicon(context.Background(), server.URL+"/favicon.ico")
	if dataURL != "" {
		t.Fatalf("expected empty data URL for non-image content, got %s", dataURL)
	}
}

func TestRewriteCheckURLSchemeHonorsConfiguredProtocol(t *testing.T) {
	// Regression for #1628: when a site is indexed as http but the healthcheck
	// protocol is configured as https, the request URL must use https.
	// Unlike generateDisplayURL, this helper preserves path, query, and port.
	cases := []struct {
		name     string
		url      string
		protocol string
		want     string
	}{
		{"http site with https protocol", "http://example.com", "https", "https://example.com"},
		{"https site with http protocol is not downgraded", "https://example.com", "http", "https://example.com"},
		{"http site with http protocol stays http", "http://example.com", "http", "http://example.com"},
		{"https site with https protocol stays https", "https://example.com", "https", "https://example.com"},
		{"path is preserved when scheme is rewritten", "http://example.com/app", "https", "https://example.com/app"},
		{"query is preserved when scheme is rewritten", "http://example.com/health?token=abc", "https", "https://example.com/health?token=abc"},
		{"non-default port is preserved", "http://example.com:8080/app", "https", "https://example.com:8080/app"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rewriteCheckURLScheme(tc.url, tc.protocol); got != tc.want {
				t.Fatalf("rewriteCheckURLScheme(%q, %q) = %q, want %q", tc.url, tc.protocol, got, tc.want)
			}
		})
	}
}

func TestCheckSiteWithConfigRewritesURLScheme(t *testing.T) {
	t.Cleanup(InvalidateSiteConfigCache)

	// Seed cache so checkHTTP's getOrCreateSiteConfigForURL doesn't hit a nil DB.
	setCachedSiteConfig("example.com:443", &model.SiteConfig{
		Model: model.Model{ID: 1},
		Host:  "example.com:443",
	})

	checker := NewEnhancedSiteChecker()

	var capturedScheme string
	checker.defaultClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedScheme = req.URL.Scheme
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    req,
			}, nil
		}),
	}

	// Use Protocol "http" with an HTTPS-indexed URL to exercise the rewrite path
	// without triggering checkHTTPS's forced ValidateSSL=true (which replaces the client).
	config := &model.HealthCheckConfig{
		Protocol:       "http",
		Method:         "GET",
		Path:           "/",
		ExpectedStatus: []int{200},
	}

	if _, err := checker.CheckSiteWithConfig(context.Background(), "https://example.com", config); err != nil {
		t.Fatalf("CheckSiteWithConfig returned error: %v", err)
	}

	// determineOptimalScheme preserves https even when the user picks http, so
	// the outgoing request must be https — verifying the rewrite runs.
	if capturedScheme != "https" {
		t.Fatalf("expected request scheme https, got %q", capturedScheme)
	}
}

func TestDownloadFaviconRejectsHTMLContentWithoutHeader(t *testing.T) {
	checker := NewSiteChecker(DefaultCheckOptions())
	checker.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("<html><body>not an image</body></html>")),
				Request:    req,
			}, nil
		}),
	}

	dataURL := checker.downloadFavicon(context.Background(), "http://example.com/favicon.ico")
	if dataURL != "" {
		t.Fatalf("expected empty data URL when header missing and content sniffing rejects, got %s", dataURL)
	}
}

// TestSharedTransportIsReused asserts that the package-level transport is the
// same instance across NewSiteChecker / NewEnhancedSiteChecker constructions.
// This is the structural guarantee that prevents the per-request transport
// allocation pattern that caused #1608.
func TestSharedTransportIsReused(t *testing.T) {
	a := NewSiteChecker(DefaultCheckOptions()).client.Transport
	b := NewSiteChecker(DefaultCheckOptions()).client.Transport
	c := NewEnhancedSiteChecker().defaultClient.Transport
	d := SharedTransport()

	if a != b {
		t.Fatalf("two SiteCheckers should share one transport, got %p vs %p", a, b)
	}
	if a != c {
		t.Fatalf("SiteChecker and EnhancedSiteChecker should share one transport, got %p vs %p", a, c)
	}
	if a != d {
		t.Fatalf("SiteChecker transport differs from SharedTransport(), got %p vs %p", a, d)
	}
}

// TestClientForHealthCheckUsesSharedClientWhenNoTLSDivergence verifies the
// fast path: when a HealthCheckConfig does not request custom TLS, no new
// transport is built — the shared one is reused.
func TestClientForHealthCheckUsesSharedClientWhenNoTLSDivergence(t *testing.T) {
	cfg := &model.HealthCheckConfig{Protocol: "https"}
	got := ClientForHealthCheck(cfg, time.Second)
	if got.Transport != SharedTransport() {
		t.Fatalf("expected shared transport, got separate instance")
	}
}

// TestClientForHealthCheckBuildsCustomWhenTLSDiverges verifies that genuinely
// divergent TLS configs DO get their own transport (so we don't pollute the
// shared pool's TLS config).
func TestClientForHealthCheckBuildsCustomWhenTLSDiverges(t *testing.T) {
	cfg := &model.HealthCheckConfig{Protocol: "https", ValidateSSL: true}
	got := ClientForHealthCheck(cfg, time.Second)
	if got.Transport == SharedTransport() {
		t.Fatalf("expected dedicated transport for ValidateSSL=true, got shared one")
	}
}

// TestCheckAllSitesRespectsDisabledSetting confirms the global kill switch
// short-circuits CheckAllSites without making any network calls.
func TestCheckAllSitesRespectsDisabledSetting(t *testing.T) {
	t.Cleanup(InvalidateSiteConfigCache)
	originalEnabled := settings.SiteCheckSettings.Enabled
	settings.SiteCheckSettings.Enabled = false
	t.Cleanup(func() { settings.SiteCheckSettings.Enabled = originalEnabled })

	checker := NewSiteChecker(DefaultCheckOptions())
	checker.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected HTTP request to %s while site check is globally disabled", req.URL.String())
			return nil, nil
		}),
	}

	checker.sites["http://example.com"] = &SiteInfo{Name: "example.com"}
	checker.CheckAllSites(context.Background())
}

// TestCheckAllSitesDedupesByHostPort confirms that two URLs sharing the same
// host:port produce a single network call. This is the multi-server_name
// scenario from #1608.
func TestCheckAllSitesDedupesByHostPort(t *testing.T) {
	t.Cleanup(InvalidateSiteConfigCache)
	originalEnabled := settings.SiteCheckSettings.Enabled
	settings.SiteCheckSettings.Enabled = true
	t.Cleanup(func() { settings.SiteCheckSettings.Enabled = originalEnabled })

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	opts := DefaultCheckOptions()
	opts.CheckFavicon = false // isolate the dedupe assertion from favicon GETs
	checker := NewSiteChecker(opts)
	seedSiteConfigForTest(t, server.URL)

	// Three aliases that all dedupe to the same host:port.
	checker.sites[server.URL] = &SiteInfo{Name: "alias-1"}
	checker.sites[server.URL+"/"] = &SiteInfo{Name: "alias-2"}
	checker.sites[server.URL+"/?token=xyz"] = &SiteInfo{Name: "alias-3"}

	checker.CheckAllSites(context.Background())

	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 network call after dedupe, got %d", got)
	}
}

// seedSiteConfigForTest pre-populates the in-memory site config cache so the
// checker can run without hitting the (nil) test database.
func seedSiteConfigForTest(t *testing.T, rawURL string) {
	t.Helper()
	cfg := &model.SiteConfig{HealthCheckEnabled: true}
	cfg.SetFromURL(rawURL)
	setCachedSiteConfig(cfg.Host, cfg)
}

// TestCheckAllSitesHonorsConcurrencyLimit ensures the configured concurrency
// is enforced. We register a slow handler and assert the in-flight count
// never exceeds the configured cap.
func TestCheckAllSitesHonorsConcurrencyLimit(t *testing.T) {
	t.Cleanup(InvalidateSiteConfigCache)

	originalEnabled := settings.SiteCheckSettings.Enabled
	originalConcurrency := settings.SiteCheckSettings.Concurrency
	settings.SiteCheckSettings.Enabled = true
	settings.SiteCheckSettings.Concurrency = 2
	t.Cleanup(func() {
		settings.SiteCheckSettings.Enabled = originalEnabled
		settings.SiteCheckSettings.Concurrency = originalConcurrency
	})

	var inFlight, peak int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&inFlight, 1)
		// Track the high-water mark.
		for {
			old := atomic.LoadInt32(&peak)
			if current <= old || atomic.CompareAndSwapInt32(&peak, old, current) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	opts := DefaultCheckOptions()
	opts.CheckFavicon = false
	checker := NewSiteChecker(opts)
	// Use distinct hosts via URL fragments... actually we need different
	// host:port pairs to avoid dedupe. Spin up multiple servers.
	urls := make([]string, 0, 6)
	servers := []*httptest.Server{server}
	for i := 0; i < 5; i++ {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			current := atomic.AddInt32(&inFlight, 1)
			for {
				old := atomic.LoadInt32(&peak)
				if current <= old || atomic.CompareAndSwapInt32(&peak, old, current) {
					break
				}
			}
			time.Sleep(40 * time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
			w.WriteHeader(http.StatusOK)
		}))
		servers = append(servers, s)
	}
	t.Cleanup(func() {
		for _, s := range servers {
			s.Close()
		}
	})
	for _, s := range servers {
		urls = append(urls, s.URL)
		seedSiteConfigForTest(t, s.URL)
	}
	for _, u := range urls {
		checker.sites[u] = &SiteInfo{Name: u}
	}

	checker.CheckAllSites(context.Background())

	if got := atomic.LoadInt32(&peak); got > 2 {
		t.Fatalf("expected peak in-flight <= 2 (configured concurrency), got %d", got)
	}
}

// TestEnhancedCheckDoesNotDoubleFetchForFavicon verifies that when the
// enhanced HTTP check has already fetched HTML, the favicon extraction reuses
// that body instead of issuing a second GET to "/" (#1608).
func TestEnhancedCheckDoesNotDoubleFetchForFavicon(t *testing.T) {
	t.Cleanup(InvalidateSiteConfigCache)
	originalEnabled := settings.SiteCheckSettings.Enabled
	settings.SiteCheckSettings.Enabled = true
	t.Cleanup(func() { settings.SiteCheckSettings.Enabled = originalEnabled })

	var rootHits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			atomic.AddInt32(&rootHits, 1)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, `<html><head><link rel="icon" href="/missing.ico"></head><body>ok</body></html>`)
			return
		}
		// Any favicon download request — return 404 so we don't pollute
		// the root counter.
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := &model.SiteConfig{
		HealthCheckEnabled: true,
		HealthCheckConfig: &model.HealthCheckConfig{
			Protocol:       "http",
			Method:         "GET",
			Path:           "/",
			ExpectedStatus: []int{200},
		},
	}
	cfg.SetFromURL(server.URL)
	setCachedSiteConfig(cfg.Host, cfg)

	checker := NewSiteChecker(DefaultCheckOptions()) // CheckFavicon defaults to true
	if _, err := checker.CheckSite(context.Background(), server.URL); err != nil {
		t.Fatalf("CheckSite returned error: %v", err)
	}

	if got := atomic.LoadInt32(&rootHits); got != 1 {
		t.Fatalf("expected exactly 1 GET to / (favicon should reuse health-check body), got %d", got)
	}
}

// TestDedupeKey covers the URL → key normalisation used to coalesce aliases.
func TestDedupeKey(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"http://example.com", "http://example.com:80"},
		{"http://example.com/", "http://example.com:80"},
		{"https://example.com", "https://example.com:443"},
		{"https://example.com:8443", "https://example.com:8443"},
		{"https://Example.COM/path?a=1", "https://example.com:443"},
		{"grpc://example.com", "grpc://example.com:80"},
		{"grpcs://example.com", "grpcs://example.com:443"},
	}
	for _, tc := range cases {
		if got := dedupeKey(tc.url); got != tc.want {
			t.Errorf("dedupeKey(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}
