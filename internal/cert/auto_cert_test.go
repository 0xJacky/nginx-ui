package cert

import (
	"context"
	stderrors "errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
)

func TestShouldSkipAutoRenew(t *testing.T) {
	now := time.Date(2026, time.April, 19, 12, 0, 0, 0, time.UTC)
	recentFailureAt := now.Add(-11 * time.Hour)
	expiredFailureAt := now.Add(-13 * time.Hour)

	tests := []struct {
		name     string
		cert     *model.Cert
		expected bool
	}{
		{
			name: "skip recent failed renewal",
			cert: &model.Cert{
				LastAutoRenewAt:    &recentFailureAt,
				LastAutoRenewError: "challenge error",
			},
			expected: true,
		},
		{
			name: "retry after cooldown window",
			cert: &model.Cert{
				LastAutoRenewAt:    &expiredFailureAt,
				LastAutoRenewError: "challenge error",
			},
			expected: false,
		},
		{
			name: "do not skip successful renewal state",
			cert: &model.Cert{
				LastAutoRenewAt:    &recentFailureAt,
				LastAutoRenewError: "",
			},
			expected: false,
		},
		{
			name: "do not skip without attempt timestamp",
			cert: &model.Cert{
				LastAutoRenewError: "challenge error",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSkipAutoRenew(tt.cert, now); got != tt.expected {
				t.Fatalf("shouldSkipAutoRenew() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBuildAutoRenewNotificationDetails(t *testing.T) {
	err := cosy.WrapErrorWithParams(ErrRenewCert, "dns token invalid")

	details := buildAutoRenewNotificationDetails("example.com", err)

	if got := details["name"]; got != "example.com" {
		t.Fatalf("unexpected name: %v", got)
	}

	if got := details["error"]; got != err.Error() {
		t.Fatalf("unexpected error text: %v", got)
	}

	response, ok := details["response"].(*cosy.Error)
	if !ok {
		t.Fatalf("unexpected response type: %T", details["response"])
	}

	if response.Scope != "cert" || response.Code != 50018 {
		t.Fatalf("unexpected cosy error payload: %+v", response)
	}
}

func TestGetAutoRenewNotificationResponseFallsBackToPlainText(t *testing.T) {
	err := stderrors.New("plain failure")

	response := getAutoRenewNotificationResponse(err)

	text, ok := response.(string)
	if !ok {
		t.Fatalf("unexpected response type: %T", response)
	}

	if text != "plain failure" {
		t.Fatalf("unexpected fallback response: %s", text)
	}
}

func TestShouldSkipAutoCertForNonSuccessStatus(t *testing.T) {
	cases := []struct {
		name     string
		status   string
		expected bool
	}{
		{"pending is skipped", model.CertStatusPending, true},
		{"failure is skipped", model.CertStatusFailure, true},
		{"success is renewed", model.CertStatusSuccess, false},
		{"empty (legacy) is renewed", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cert := &model.Cert{Status: tc.status}
			if got := shouldSkipAutoCertByStatus(cert); got != tc.expected {
				t.Fatalf("shouldSkipAutoCertByStatus(%q) = %v, want %v", tc.status, got, tc.expected)
			}
		})
	}
}

func TestNewAutoRenewPayloadPreservesChallengeConfig(t *testing.T) {
	notBefore := time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC)
	certModel := &model.Cert{
		ChallengeConfig: map[string]any{"disable_authoritative_ns_propagation": true},
	}

	payload := newAutoRenewPayload(certModel, &Info{NotBefore: notBefore}, "aki.serial")

	if payload.ChallengeConfig["disable_authoritative_ns_propagation"] != true {
		t.Fatal("challenge config was not copied to auto-renew payload")
	}
	if !payload.NotBefore.Equal(notBefore) {
		t.Fatalf("NotBefore = %s, want %s", payload.NotBefore, notBefore)
	}
	if payload.ReplacesCertID != "aki.serial" {
		t.Fatalf("ReplacesCertID = %q, want aki.serial", payload.ReplacesCertID)
	}
}

func TestNewAutoRenewPayloadSkipsPreIssuanceRouteCheck(t *testing.T) {
	certModel := &model.Cert{Filename: "example.com", Domains: []string{"example.com"}}

	payload := newAutoRenewPayload(certModel, &Info{}, "")

	// A non-empty ConfigName would make IssueCert run the HTTP-01 route check
	// before renewing; unattended renewals only probe after a failure.
	if payload.ConfigName != "" {
		t.Fatalf("ConfigName = %q, want empty for auto-renew", payload.ConfigName)
	}
}

func TestAppendHTTP01ProbeSummaryAfterRenewalFailure(t *testing.T) {
	previous := autoRenewHTTP01Probe
	t.Cleanup(func() { autoRenewHTTP01Probe = previous })

	var gotDomains []string
	autoRenewHTTP01Probe = func(_ context.Context, domains []string, _ ...HTTP01ProbeOption) ([]HTTP01ProbeResult, error) {
		gotDomains = domains
		return []HTTP01ProbeResult{{
			Domain:     "example.com",
			Status:     HTTP01ProbeStatusFailure,
			Target:     "127.0.0.1:80",
			StatusCode: 404,
			Error:      "unexpected status 404",
		}}, nil
	}
	renewErr := cosy.WrapErrorWithParams(ErrRenewCert, "acme: error: 403")

	err := appendHTTP01ProbeSummary(renewErr, []string{"example.com"}, "example.com", nil)

	if len(gotDomains) != 1 || gotDomains[0] != "example.com" {
		t.Fatalf("probe domains = %v", gotDomains)
	}
	if !stderrors.Is(err, renewErr) {
		t.Fatalf("original error is not wrapped: %v", err)
	}
	if !strings.Contains(err.Error(), "HTTP-01 route check: example.com: unexpected status 404 (via 127.0.0.1:80)") {
		t.Fatalf("summary missing from error: %s", err.Error())
	}
	response, ok := buildAutoRenewNotificationDetails("example.com", err)["response"].(*cosy.Error)
	if !ok || response.Code != 50018 {
		t.Fatalf("notification response lost the cosy error: %#v", response)
	}
}

func TestAppendHTTP01ProbeSummaryWhenProbeCannotRun(t *testing.T) {
	previous := autoRenewHTTP01Probe
	t.Cleanup(func() { autoRenewHTTP01Probe = previous })
	autoRenewHTTP01Probe = func(context.Context, []string, ...HTTP01ProbeOption) ([]HTTP01ProbeResult, error) {
		return nil, NewHTTP01ChallengePortUnavailableError("9180", "address already in use")
	}

	err := appendHTTP01ProbeSummary(stderrors.New("renew failed"), []string{"example.com"}, "", nil)

	if !strings.Contains(err.Error(), "HTTP-01 route check: not run: HTTP-01 challenge port 9180 is unavailable") {
		t.Fatalf("unexpected error text: %s", err.Error())
	}
}

func TestAppendHTTP01ProbeSummaryDiscoversEndpointsPerDomain(t *testing.T) {
	// The real probe runs: the certificate file name matches no site, and
	// the SAN is served by a block on a specific address only.
	fake, _ := setupHTTP01Probe(t, map[string]http.Handler{"api.example.com": http.NotFoundHandler()}, nil)
	useHTTP01Blocks([]nginx.ServerBlock{{
		File:        "/etc/nginx/sites-enabled/api",
		ServerNames: []string{"api.example.com"},
		Listens:     []nginx.ServerListen{{Addr: "192.0.2.10", Port: "80"}},
	}}, map[string]string{"192.0.2.10:80": fake.endpoint().HTTP})

	err := appendHTTP01ProbeSummary(stderrors.New("renew failed"), []string{"api.example.com"}, "shared-cert", nil)

	want := "api.example.com: unexpected status 404 (via " + fake.endpoint().HTTP + "); possible cause: the server block for api.example.com on 192.0.2.10:80 in /etc/nginx/sites-enabled/api has no /.well-known/acme-challenge location"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("summary does not come from the discovered endpoint: %s", err.Error())
	}
}

func TestShouldRenewACMECertificateRenewsShortLifetimeAtMidpoint(t *testing.T) {
	notBefore := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	info := &Info{
		NotBefore: notBefore,
		NotAfter:  notBefore.Add(160 * time.Hour),
	}

	if shouldRenewACMECertificate(info, notBefore.Add(79*time.Hour), 7) {
		t.Fatal("certificate renewed before half of its lifetime elapsed")
	}
	if !shouldRenewACMECertificate(info, notBefore.Add(80*time.Hour), 7) {
		t.Fatal("certificate not renewed at half of its lifetime")
	}
}

func TestShouldRenewACMECertificateUsesRemainingValidityThreshold(t *testing.T) {
	notBefore := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	info := &Info{
		NotBefore: notBefore,
		NotAfter:  notBefore.Add(90 * 24 * time.Hour),
	}

	if shouldRenewACMECertificate(info, info.NotAfter.Add(-31*24*time.Hour), 30) {
		t.Fatal("normal certificate renewed with more than the configured validity remaining")
	}
	if !shouldRenewACMECertificate(info, info.NotAfter.Add(-30*24*time.Hour), 30) {
		t.Fatal("normal certificate not renewed at the remaining-validity threshold")
	}
}

func TestCertificateRenewalTimeUsesMidpointForOversizedThreshold(t *testing.T) {
	notBefore := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	info := &Info{
		NotBefore: notBefore,
		NotAfter:  notBefore.Add(20 * 24 * time.Hour),
	}

	want := notBefore.Add(10 * 24 * time.Hour)
	if got := certificateRenewalTime(info, 30); !got.Equal(want) {
		t.Fatalf("certificateRenewalTime() = %s, want %s", got, want)
	}
}
