package sitecheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
)

// newSelfSignedSite starts an HTTPS test server whose certificate is generated
// by httptest and therefore signed by an authority the process does not trust.
// This mirrors an internal-only vhost using a self-signed certificate or a
// private CA, which is the setup reported in #1790.
func newSelfSignedSite(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	return server
}

// --- probeVerifiesCertificate case 4: plain liveness probe ----------------

// TestCheckSiteAcceptsSelfSignedCertificate is the regression test for #1790:
// a site serving a self-signed certificate is alive, and a credential-free
// probe must report it online instead of failing certificate verification.
func TestCheckSiteAcceptsSelfSignedCertificate(t *testing.T) {
	t.Cleanup(InvalidateSiteConfigCache)
	setGlobalInsecureSkipVerify(t, false)

	originalEnabled := settings.SiteCheckSettings.Enabled
	settings.SiteCheckSettings.Enabled = true
	t.Cleanup(func() { settings.SiteCheckSettings.Enabled = originalEnabled })

	server := newSelfSignedSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := &model.SiteConfig{
		HealthCheckEnabled: true,
		HealthCheckConfig: &model.HealthCheckConfig{
			Protocol:       "https",
			Method:         http.MethodGet,
			Path:           "/",
			ExpectedStatus: []int{http.StatusOK},
		},
	}
	cfg.SetFromURL(server.URL)
	setCachedSiteConfig(canonicalSiteKey("", server.URL), cfg)

	opts := DefaultCheckOptions()
	opts.CheckFavicon = false
	checker := NewSiteChecker(opts)

	info, err := checker.CheckSite(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("CheckSite returned error: %v", err)
	}
	if info == nil {
		t.Fatal("CheckSite returned no site info")
	}
	if info.Status != StatusOnline {
		t.Fatalf("expected self-signed site to be %q, got %q (error: %q, type: %q)",
			StatusOnline, info.Status, info.Error, info.ErrorType)
	}
	if info.Error != "" {
		t.Fatalf("expected no error for a healthy self-signed site, got %q", info.Error)
	}
}

// TestCheckSiteBasicAcceptsSelfSignedCertificate covers the fallback path used
// when no health check configuration is stored for the site.
func TestCheckSiteBasicAcceptsSelfSignedCertificate(t *testing.T) {
	t.Cleanup(InvalidateSiteConfigCache)
	setGlobalInsecureSkipVerify(t, false)

	server := newSelfSignedSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Seed the cache so getOrCreateSiteConfigForURL does not reach the nil
	// test database.
	seedSiteConfigForTest(t, server.URL)

	opts := DefaultCheckOptions()
	opts.CheckFavicon = false
	checker := NewSiteChecker(opts)

	info, err := checker.checkSiteBasic(context.Background(), "", server.URL, "https")
	if err != nil {
		t.Fatalf("checkSiteBasic returned error: %v", err)
	}
	if info.Status != StatusOnline {
		t.Fatalf("expected self-signed site to be %q, got %q (error: %q)", StatusOnline, info.Status, info.Error)
	}
}

// TestEnhancedCheckAcceptsSelfSignedCertificate asserts the enhanced checker
// used by the "test health check" API also tolerates self-signed certificates.
func TestEnhancedCheckAcceptsSelfSignedCertificate(t *testing.T) {
	setGlobalInsecureSkipVerify(t, false)

	server := newSelfSignedSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	checker := NewEnhancedSiteChecker()
	result, err := checker.CheckSiteWithConfig(context.Background(), server.URL, &model.HealthCheckConfig{
		Protocol:       "https",
		Method:         http.MethodGet,
		Path:           "/",
		ExpectedStatus: []int{http.StatusOK},
	})
	if err != nil {
		t.Fatalf("CheckSiteWithConfig returned error: %v", err)
	}
	if result == nil || result.Info == nil {
		t.Fatal("CheckSiteWithConfig returned no result")
	}
	if result.Info.Status != StatusOnline {
		t.Fatalf("expected self-signed site to be %q, got %q (error: %q)",
			StatusOnline, result.Info.Status, result.Info.Error)
	}
}

// setGlobalInsecureSkipVerify overrides the global HTTP setting for one test
// and restores it afterwards, so the four probeVerifiesCertificate cases can be
// exercised independently of test ordering.
func setGlobalInsecureSkipVerify(t *testing.T, value bool) {
	t.Helper()
	original := settings.HTTPSettings.InsecureSkipVerify
	settings.HTTPSettings.InsecureSkipVerify = value
	t.Cleanup(func() { settings.HTTPSettings.InsecureSkipVerify = original })
}

// probeSelfSignedSite runs an HTTPS health check against a self-signed server
// and returns the resulting SiteInfo.
func probeSelfSignedSite(t *testing.T, cfg *model.HealthCheckConfig) *SiteInfo {
	t.Helper()

	server := newSelfSignedSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	result, _ := NewEnhancedSiteChecker().CheckSiteWithConfig(context.Background(), server.URL, cfg)
	if result == nil || result.Info == nil {
		t.Fatal("CheckSiteWithConfig returned no result")
	}
	return result.Info
}

// httpsProbeConfig builds a minimal HTTPS health check configuration.
func httpsProbeConfig() *model.HealthCheckConfig {
	return &model.HealthCheckConfig{
		Protocol:       "https",
		Method:         http.MethodGet,
		Path:           "/",
		ExpectedStatus: []int{http.StatusOK},
	}
}

// --- probeVerifiesCertificate case 1: explicit per-site opt-in -------------

// TestValidateSSLStillRejectsSelfSignedCertificate guards the opt-in path: an
// operator who explicitly asks the probe to validate certificates must still
// get a failure for an untrusted chain, with a TLS-typed error.
func TestValidateSSLStillRejectsSelfSignedCertificate(t *testing.T) {
	setGlobalInsecureSkipVerify(t, false)

	cfg := httpsProbeConfig()
	cfg.ValidateSSL = true

	info := probeSelfSignedSite(t, cfg)
	if info.Status == StatusOnline {
		t.Fatal("expected ValidateSSL=true to reject an untrusted certificate")
	}
	if info.ErrorType != ErrorTypeTLS {
		t.Fatalf("expected error type %q, got %q (error: %q)", ErrorTypeTLS, info.ErrorType, info.Error)
	}
}

// TestValidateSSLOutranksGlobalInsecureSkipVerify pins the case ordering: an
// explicit per-site opt-in must not be undone by the global switch.
func TestValidateSSLOutranksGlobalInsecureSkipVerify(t *testing.T) {
	setGlobalInsecureSkipVerify(t, true)

	cfg := httpsProbeConfig()
	cfg.ValidateSSL = true

	info := probeSelfSignedSite(t, cfg)
	if info.Status == StatusOnline {
		t.Fatal("ValidateSSL=true must still verify even when the global switch skips verification")
	}
	if info.ErrorType != ErrorTypeTLS {
		t.Fatalf("expected error type %q, got %q (error: %q)", ErrorTypeTLS, info.ErrorType, info.Error)
	}
}

// --- probeVerifiesCertificate case 2: global opt-out ----------------------

// TestGlobalInsecureSkipVerifyCoversProbesWithHeaders is the escape hatch for
// case 3: an operator who globally declared "never verify" gets a self-signed
// site reported as online even though the probe sends custom headers.
func TestGlobalInsecureSkipVerifyCoversProbesWithHeaders(t *testing.T) {
	setGlobalInsecureSkipVerify(t, true)

	cfg := httpsProbeConfig()
	cfg.Headers = map[string]string{"Authorization": "Bearer probe-token"}

	info := probeSelfSignedSite(t, cfg)
	if info.Status != StatusOnline {
		t.Fatalf("expected global InsecureSkipVerify to make the probe succeed, got %q (error: %q, type: %q)",
			info.Status, info.Error, info.ErrorType)
	}
}

// --- probeVerifiesCertificate case 3: operator-supplied data --------------

// TestProbeWithCustomHeadersVerifiesCertificate proves end to end that a probe
// carrying operator-supplied headers refuses to talk to an unverified peer.
// A bearer token in HealthCheckConfig.Headers must never reach a server whose
// certificate was not validated, so this probe fails against a self-signed
// site rather than leaking the credential.
func TestProbeWithCustomHeadersVerifiesCertificate(t *testing.T) {
	setGlobalInsecureSkipVerify(t, false)

	cfg := httpsProbeConfig()
	cfg.Headers = map[string]string{"Authorization": "Bearer probe-token"}

	info := probeSelfSignedSite(t, cfg)
	if info.Status == StatusOnline {
		t.Fatal("a probe carrying custom headers must not accept an unverified certificate")
	}
	if info.ErrorType != ErrorTypeTLS {
		t.Fatalf("expected error type %q, got %q (error: %q)", ErrorTypeTLS, info.ErrorType, info.Error)
	}
}

// TestProbeWithCustomHeadersNeverReachesUnverifiedPeer is the direct evidence
// for the previous test: the self-signed server records every request it
// serves, and must never see the probe's Authorization header.
func TestProbeWithCustomHeadersNeverReachesUnverifiedPeer(t *testing.T) {
	setGlobalInsecureSkipVerify(t, false)

	var seenAuthorization atomic.Value
	seenAuthorization.Store("")
	server := newSelfSignedSite(t, func(w http.ResponseWriter, r *http.Request) {
		seenAuthorization.Store(r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	})

	cfg := httpsProbeConfig()
	cfg.Headers = map[string]string{"Authorization": "Bearer probe-token"}

	if _, err := NewEnhancedSiteChecker().CheckSiteWithConfig(context.Background(), server.URL, cfg); err == nil {
		t.Fatal("expected the probe to fail against an unverified peer")
	}

	if got := seenAuthorization.Load().(string); got != "" {
		t.Fatalf("Authorization header leaked to an unverified peer: %q", got)
	}
}

// TestProbeWithRequestBodyVerifiesCertificate covers the other half of case 3:
// a configured request body is operator-supplied data just like a header.
func TestProbeWithRequestBodyVerifiesCertificate(t *testing.T) {
	setGlobalInsecureSkipVerify(t, false)

	cfg := httpsProbeConfig()
	cfg.Method = http.MethodPost
	cfg.Body = `{"probe":"payload"}`

	info := probeSelfSignedSite(t, cfg)
	if info.Status == StatusOnline {
		t.Fatal("a probe carrying a request body must not accept an unverified certificate")
	}
	if info.ErrorType != ErrorTypeTLS {
		t.Fatalf("expected error type %q, got %q (error: %q)", ErrorTypeTLS, info.ErrorType, info.Error)
	}
}

// --- the decision table itself -------------------------------------------

// TestProbeVerifiesCertificate walks every branch of the decision, including
// the combinations where one case has to outrank another.
func TestProbeVerifiesCertificate(t *testing.T) {
	headers := map[string]string{"Authorization": "Bearer probe-token"}

	cases := []struct {
		name         string
		globalSkip   bool
		cfg          *model.HealthCheckConfig
		wantVerify   bool
		wantCustomer bool // expected needsCustomTLS result
	}{
		{"case 1: ValidateSSL opts in", false, &model.HealthCheckConfig{ValidateSSL: true}, true, true},
		{"case 1 outranks case 2", true, &model.HealthCheckConfig{ValidateSSL: true}, true, true},
		{"case 1 outranks case 3", false, &model.HealthCheckConfig{ValidateSSL: true, Headers: headers}, true, true},
		{"case 2: global skip", true, &model.HealthCheckConfig{}, false, false},
		{"case 2 outranks case 3", true, &model.HealthCheckConfig{Headers: headers}, false, false},
		{"case 3: custom headers", false, &model.HealthCheckConfig{Headers: headers}, true, true},
		{"case 3: request body", false, &model.HealthCheckConfig{Body: `{"a":1}`}, true, true},
		{"case 3: blank body is not data", false, &model.HealthCheckConfig{Body: "   "}, false, false},
		{"case 4: plain liveness probe", false, &model.HealthCheckConfig{Protocol: "https"}, false, false},
		{"case 4: nil config", false, nil, false, false},
		{"case 4 plus client certificate", false, &model.HealthCheckConfig{ClientCert: "c.pem", ClientKey: "k.pem"}, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setGlobalInsecureSkipVerify(t, tc.globalSkip)

			if got := probeVerifiesCertificate(tc.cfg); got != tc.wantVerify {
				t.Fatalf("probeVerifiesCertificate() = %v, want %v", got, tc.wantVerify)
			}
			if got := needsCustomTLS(tc.cfg); got != tc.wantCustomer {
				t.Fatalf("needsCustomTLS() = %v, want %v", got, tc.wantCustomer)
			}
		})
	}
}

// TestSharedTransportSkipsCertificateVerification pins the transport behind
// cases 2 and 4 so a future change cannot silently reintroduce #1790.
func TestSharedTransportSkipsCertificateVerification(t *testing.T) {
	setGlobalInsecureSkipVerify(t, false)

	tlsConfig := SharedTransport().TLSClientConfig
	if tlsConfig == nil || !tlsConfig.InsecureSkipVerify {
		t.Fatal("the credential-free probe transport must skip certificate verification")
	}
}

// TestVerifyingTransportValidatesCertificates pins the transport behind cases
// 1 and 3, and that it is pooled rather than allocated per probe.
func TestVerifyingTransportValidatesCertificates(t *testing.T) {
	tlsConfig := VerifyingTransport().TLSClientConfig
	if tlsConfig == nil || tlsConfig.InsecureSkipVerify {
		t.Fatal("the verifying probe transport must validate certificates")
	}
	if VerifyingTransport() == SharedTransport() {
		t.Fatal("verifying and skip-verify probes must not share one transport")
	}
	if VerifyingTransport() != VerifyingTransport() {
		t.Fatal("the verifying transport must be pooled, not rebuilt per call")
	}
}

// TestClientForHealthCheckReusesPooledTransports confirms that neither case 1
// nor case 3 allocates a fresh transport per probe; only a client certificate,
// which cannot be shared, still does.
func TestClientForHealthCheckReusesPooledTransports(t *testing.T) {
	setGlobalInsecureSkipVerify(t, false)

	headers := map[string]string{"Authorization": "Bearer probe-token"}
	verifying := []*model.HealthCheckConfig{
		{Protocol: "https", ValidateSSL: true},
		{Protocol: "https", Headers: headers},
	}
	for _, cfg := range verifying {
		if got := ClientForHealthCheck(cfg, time.Second); got.Transport != VerifyingTransport() {
			t.Fatalf("expected the pooled verifying transport for %#v", cfg)
		}
	}

	if got := ClientForHealthCheck(&model.HealthCheckConfig{Protocol: "https"}, time.Second); got.Transport != SharedTransport() {
		t.Fatal("expected the pooled skip-verify transport for a plain liveness probe")
	}

	withCert := &model.HealthCheckConfig{Protocol: "https", ClientCert: "c.pem", ClientKey: "k.pem"}
	got := ClientForHealthCheck(withCert, time.Second)
	if got.Transport == SharedTransport() || got.Transport == VerifyingTransport() {
		t.Fatal("a client certificate must get its own transport")
	}
}

// TestGRPCTLSConfigFollowsProbeDecision confirms the gRPC path reuses the same
// four-case decision instead of maintaining its own.
func TestGRPCTLSConfigFollowsProbeDecision(t *testing.T) {
	headers := map[string]string{"Authorization": "Bearer probe-token"}

	cases := []struct {
		name       string
		globalSkip bool
		cfg        *model.HealthCheckConfig
		wantSkip   bool
	}{
		{"case 1: ValidateSSL verifies", false, &model.HealthCheckConfig{Protocol: "grpcs", ValidateSSL: true}, false},
		{"case 2: global skip", true, &model.HealthCheckConfig{Protocol: "grpcs"}, true},
		{"case 3: custom headers verify", false, &model.HealthCheckConfig{Protocol: "grpcs", Headers: headers}, false},
		{"case 4: plain liveness probe skips", false, &model.HealthCheckConfig{Protocol: "grpcs"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setGlobalInsecureSkipVerify(t, tc.globalSkip)

			if got := grpcTLSConfig(tc.cfg).InsecureSkipVerify; got != tc.wantSkip {
				t.Fatalf("grpcTLSConfig().InsecureSkipVerify = %v, want %v", got, tc.wantSkip)
			}
		})
	}
}

func TestClassifyCheckError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{
			"untrusted chain",
			&net.OpError{Op: "remote error", Err: &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}},
			ErrorTypeTLS,
		},
		{"hostname mismatch", x509.HostnameError{Host: "example.com"}, ErrorTypeTLS},
		{"expired certificate", x509.CertificateInvalidError{Reason: x509.Expired}, ErrorTypeTLS},
		{"plaintext on https port", tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"}, ErrorTypeTLS},
		{"dns failure", &net.DNSError{Err: "no such host", Name: "internal.example.com"}, ErrorTypeDNS},
		{"connection refused", &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}, ErrorTypeConnectionRefused},
		{"deadline exceeded", context.DeadlineExceeded, ErrorTypeTimeout},
		{"other transport failure", errors.New("broken pipe"), ErrorTypeNetwork},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyCheckError(tc.err); got != tc.want {
				t.Fatalf("classifyCheckError(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

// TestClassifyCheckErrorDetectsTimeout uses a real net.Error so the net.Error
// branch is exercised rather than only the context sentinel.
func TestClassifyCheckErrorDetectsTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
		t.Fatalf("failed to set deadline: %v", err)
	}
	buf := make([]byte, 1)
	_, readErr := conn.Read(buf)
	if readErr == nil {
		t.Skip("read unexpectedly succeeded; timeout could not be provoked")
	}

	if got := classifyCheckError(readErr); got != ErrorTypeTimeout {
		t.Fatalf("classifyCheckError(%v) = %q, want %q", readErr, got, ErrorTypeTimeout)
	}
}
