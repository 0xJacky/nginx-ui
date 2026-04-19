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
