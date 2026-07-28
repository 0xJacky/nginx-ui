package cert

import (
	stderrors "errors"
	"testing"
	"time"

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
