package cert

import (
	"crypto/x509"
	"math/big"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
)

func TestSelectARIRenewalTimeIsStableWithinWindow(t *testing.T) {
	certificate := testShortLivedCertificate()
	now := certificate.NotBefore.Add(24 * time.Hour)
	start := certificate.NotBefore.Add(72 * time.Hour)
	end := start.Add(time.Hour)

	first, err := selectARIRenewalTime(certificate, start, end, now)
	if err != nil {
		t.Fatalf("selectARIRenewalTime() error = %v", err)
	}
	second, err := selectARIRenewalTime(certificate, start, end, now)
	if err != nil {
		t.Fatalf("selectARIRenewalTime() second error = %v", err)
	}
	if !first.Equal(second) {
		t.Fatalf("renewal times differ: %s and %s", first, second)
	}
	if first.Before(start) || !first.Before(end) {
		t.Fatalf("renewal time %s is outside [%s, %s)", first, start, end)
	}
}

func TestGetRenewalScheduleDecisionUsesCachedARI(t *testing.T) {
	certificate := testShortLivedCertificate()
	now := certificate.NotBefore.Add(72 * time.Hour)
	renewAt := now.Add(time.Hour)
	checkedAt := now.Add(-time.Hour)
	certModel := &model.Cert{
		NextAutoRenewAt:              &renewAt,
		LastRenewalInfoCheckAt:       &checkedAt,
		AutoRenewScheduleFingerprint: certificateFingerprint(certificate),
	}

	decision := getRenewalScheduleDecision(certModel, certificate, now)
	if !decision.UsesARI || decision.Due {
		t.Fatalf("decision = %+v, want cached ARI schedule not due", decision)
	}

	now = renewAt
	decision = getRenewalScheduleDecision(certModel, certificate, now)
	if !decision.UsesARI || !decision.Due || decision.ReplacesCertID == "" {
		t.Fatalf("decision = %+v, want due ARI replacement", decision)
	}
}

func testShortLivedCertificate() *x509.Certificate {
	notBefore := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	return &x509.Certificate{
		Raw:            []byte("short-lived-certificate"),
		SerialNumber:   big.NewInt(42),
		AuthorityKeyId: []byte("authority-key"),
		NotBefore:      notBefore,
		NotAfter:       notBefore.Add(160 * time.Hour),
	}
}
