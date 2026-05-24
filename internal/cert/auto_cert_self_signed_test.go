package cert

import (
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
)

func TestShouldRenewSelfSignedCert(t *testing.T) {
	now := time.Date(2026, time.May, 18, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		notBefore       time.Time
		notAfter        time.Time
		renewalInterval int
		expected        bool
	}{
		{
			name:            "normal cert young enough is not renewed",
			notBefore:       now.AddDate(0, 0, -3),
			notAfter:        now.AddDate(0, 0, 362),
			renewalInterval: 7,
			expected:        false,
		},
		{
			name:            "normal cert past renewal interval is renewed",
			notBefore:       now.AddDate(0, 0, -10),
			notAfter:        now.AddDate(0, 0, 355),
			renewalInterval: 7,
			expected:        true,
		},
		{
			name:            "short-lived cert with plenty of life left is not renewed",
			notBefore:       now.AddDate(0, 0, -1),
			notAfter:        now.AddDate(0, 0, 4),
			renewalInterval: 7,
			expected:        false,
		},
		{
			name:            "short-lived cert near expiry is renewed",
			notBefore:       now.AddDate(0, 0, -4),
			notAfter:        now.AddDate(0, 0, 1),
			renewalInterval: 7,
			expected:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &Info{NotBefore: tt.notBefore, NotAfter: tt.notAfter}
			if got := shouldRenewSelfSignedCert(info, now, tt.renewalInterval); got != tt.expected {
				t.Fatalf("shouldRenewSelfSignedCert() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSelfSignedRenewalDueRejectsMissingConfig(t *testing.T) {
	_, err := selfSignedRenewalDue(&model.Cert{
		AutoCert:           model.AutoCertSelfSigned,
		SelfSignedConfig:   nil,
		SSLCertificatePath: "unused.pem",
	}, time.Now(), 7)
	if err == nil {
		t.Fatalf("expected missing self-signed config to fail renewal")
	}
}
