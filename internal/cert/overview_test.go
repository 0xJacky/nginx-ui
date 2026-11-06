package cert

import (
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
)

func TestBuildOverviewStates(t *testing.T) {
	now := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	earlier := now.Add(-time.Hour)
	later := now.Add(-time.Minute)

	infoValidFor := func(d time.Duration) *Info {
		return &Info{NotBefore: now.Add(-60 * 24 * time.Hour), NotAfter: now.Add(d)}
	}

	tests := []struct {
		name  string
		cert  *model.Cert
		info  *Info
		state State
	}{
		{"valid", &model.Cert{AutoCert: model.AutoCertEnabled, ChallengeMethod: DNS01}, infoValidFor(62 * 24 * time.Hour), StateValid},
		{"expiring", &model.Cert{AutoCert: model.AutoCertEnabled}, infoValidFor(12 * 24 * time.Hour), StateExpiring},
		{"expired", &model.Cert{AutoCert: model.AutoCertEnabled}, infoValidFor(-time.Hour), StateExpired},
		{"not issued without a file", &model.Cert{AutoCert: model.AutoCertEnabled}, nil, StateNotIssued},
		{"issuing", &model.Cert{Status: model.CertStatusPending}, nil, StateIssuing},
		{"issue failed", &model.Cert{Status: model.CertStatusFailure, LastError: "boom", LastAttemptAt: &earlier}, nil, StateFailed},
		{"manual", &model.Cert{AutoCert: model.AutoCertDisabled}, infoValidFor(200 * 24 * time.Hour), StateManual},
		{"manual expiring stays expiring", &model.Cert{AutoCert: model.AutoCertDisabled}, infoValidFor(3 * 24 * time.Hour), StateExpiring},
		{
			"auto renewal failed",
			&model.Cert{AutoCert: model.AutoCertEnabled, Status: model.CertStatusSuccess, LastAttemptAt: &earlier, LastAutoRenewAt: &later, LastAutoRenewError: "dns"},
			infoValidFor(20 * 24 * time.Hour),
			StateFailed,
		},
		{
			"later success clears an older auto renewal failure",
			&model.Cert{AutoCert: model.AutoCertEnabled, Status: model.CertStatusSuccess, LastAttemptAt: &later, LastAutoRenewAt: &earlier, LastAutoRenewError: "dns"},
			infoValidFor(80 * 24 * time.Hour),
			StateValid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := BuildOverview(test.cert, test.info, now, 30)
			if got.State != test.state {
				t.Fatalf("state = %q, want %q", got.State, test.state)
			}
		})
	}
}

func TestBuildOverviewDaysAndRenewAt(t *testing.T) {
	now := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	info := &Info{NotBefore: now.Add(-28 * 24 * time.Hour), NotAfter: now.Add(62*24*time.Hour + time.Hour)}

	got := BuildOverview(&model.Cert{AutoCert: model.AutoCertEnabled, ChallengeMethod: HTTP01}, info, now, 30)
	if got.DaysLeft == nil || *got.DaysLeft != 62 {
		t.Fatalf("days left = %v, want 62", got.DaysLeft)
	}
	if got.RenewAt == nil || !got.RenewAt.Equal(info.NotAfter.Add(-30*24*time.Hour)) {
		t.Fatalf("renew at = %v, want 30 days before expiry", got.RenewAt)
	}
	if got.RenewalMethod != RenewalMethodHTTP01 {
		t.Fatalf("renewal method = %q", got.RenewalMethod)
	}

	manual := BuildOverview(&model.Cert{AutoCert: model.AutoCertDisabled}, info, now, 30)
	if manual.RenewAt != nil {
		t.Fatal("a certificate without automatic renewal has no renewal date")
	}
	if manual.RenewalMethod != RenewalMethodUpload {
		t.Fatalf("renewal method = %q, want upload", manual.RenewalMethod)
	}

	expired := BuildOverview(&model.Cert{}, &Info{NotAfter: now.Add(-36 * time.Hour)}, now, 30)
	if expired.DaysLeft == nil || *expired.DaysLeft != -2 {
		t.Fatalf("days left = %v, want -2", expired.DaysLeft)
	}
}

func TestOverviewMatchesFilter(t *testing.T) {
	now := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	expiredInfo := &Info{NotAfter: now.Add(-time.Hour)}
	failed := BuildOverview(&model.Cert{Status: model.CertStatusFailure, LastAttemptAt: &now}, expiredInfo, now, 30)

	if !failed.MatchesFilter(FilterFailed, expiredInfo, now) || !failed.MatchesFilter(FilterExpired, expiredInfo, now) {
		t.Fatal("an expired certificate whose renewal failed belongs to both filters")
	}
	if failed.MatchesFilter(FilterExpiring, expiredInfo, now) {
		t.Fatal("an expired certificate is not expiring")
	}

	notIssued := BuildOverview(&model.Cert{AutoCert: model.AutoCertEnabled}, nil, now, 30)
	for _, filter := range []ListFilter{FilterExpiring, FilterFailed, FilterExpired} {
		if notIssued.MatchesFilter(filter, nil, now) {
			t.Fatalf("a certificate that was never issued matched %q", filter)
		}
	}

	if ParseListFilter("bogus") != FilterAll || ParseListFilter("expired") != FilterExpired {
		t.Fatal("unexpected filter parsing")
	}
}

func TestCanSwitchAutoRenewal(t *testing.T) {
	acme := &model.Cert{ChallengeMethod: HTTP01, Domains: []string{"example.com"}, AutoCert: model.AutoCertDisabled}
	if !CanSwitchAutoRenewal(acme) {
		t.Fatal("an ACME certificate can switch automatic renewal")
	}
	for _, c := range []*model.Cert{
		{AutoCert: model.AutoCertDisabled, Domains: []string{"example.com"}},
		{ChallengeMethod: HTTP01, Domains: []string{"example.com"}, AutoCert: model.AutoCertSelfSigned},
		{ChallengeMethod: HTTP01, Domains: []string{"example.com"}, AutoCert: model.AutoCertSync},
		{ChallengeMethod: HTTP01, AutoCert: model.AutoCertEnabled},
	} {
		if CanSwitchAutoRenewal(c) {
			t.Fatalf("unexpectedly switchable: %+v", c)
		}
	}
}
