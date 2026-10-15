package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
)

func TestCanUseLegoRenewDisablesRenewWhenCommonNameEnabled(t *testing.T) {
	if !canUseLegoRenew(&ConfigPayload{}) {
		t.Fatalf("canUseLegoRenew without common name = false, want true")
	}

	if canUseLegoRenew(&ConfigPayload{EnableCommonName: true}) {
		t.Fatalf("canUseLegoRenew with common name = true, want false")
	}

	if canUseLegoRenew(&ConfigPayload{ReplacesCertID: "aki.serial"}) {
		t.Fatalf("canUseLegoRenew with ARI replacement = true, want false")
	}
}

func TestDNS01ChallengeOptionsUseFixedWaitWhenAuthoritativeCheckIsDisabled(t *testing.T) {
	if got := len(dns01ChallengeOptions(&ConfigPayload{})); got != 1 {
		t.Fatalf("default option count = %d, want 1", got)
	}
	if got := dns01PropagationWait(&ConfigPayload{}); got != 0 {
		t.Fatalf("default propagation wait = %s, want 0", got)
	}

	payload := &ConfigPayload{DisableAuthoritativeNSPropagation: true}
	if got := len(dns01ChallengeOptions(payload)); got != 1 {
		t.Fatalf("disabled authoritative propagation option count = %d, want 1", got)
	}
	if got := dns01PropagationWait(payload); got != time.Minute {
		t.Fatalf("disabled authoritative propagation wait = %s, want 1m", got)
	}
}

// selfSignedPEM returns a PEM certificate for the given DNS names, with the
// first one as the common name.
func selfSignedPEM(t *testing.T, names ...string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: names[0]},
		DNSNames:     names,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// A reissue after a domain was added must not go through lego's Renew, which
// would request the identifiers of the old certificate again.
func TestRenewKeepsIdentifiersRequiresTheSameIdentifiers(t *testing.T) {
	issued := &model.CertificateResource{Certificate: selfSignedPEM(t, "a.example.com", "b.example.com")}

	cases := []struct {
		name       string
		serverName []string
		want       bool
	}{
		{name: "same", serverName: []string{"a.example.com", "b.example.com"}, want: true},
		{name: "reordered and cased", serverName: []string{"B.example.com", "a.example.com"}, want: true},
		{name: "added", serverName: []string{"a.example.com", "b.example.com", "c.example.com"}, want: false},
		{name: "removed", serverName: []string{"a.example.com"}, want: false},
		{name: "replaced", serverName: []string{"a.example.com", "c.example.com"}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := &ConfigPayload{ServerName: tc.serverName, Resource: issued}
			if got := renewKeepsIdentifiers(payload); got != tc.want {
				t.Fatalf("renewKeepsIdentifiers(%v) = %v, want %v", tc.serverName, got, tc.want)
			}
		})
	}

	if renewKeepsIdentifiers(&ConfigPayload{ServerName: []string{"a.example.com"}}) {
		t.Fatal("renewKeepsIdentifiers without a resource = true, want false")
	}
	if renewKeepsIdentifiers(&ConfigPayload{ServerName: []string{"a.example.com"}, Resource: &model.CertificateResource{Certificate: []byte("garbage")}}) {
		t.Fatal("renewKeepsIdentifiers with an unparsable certificate = true, want false")
	}
}
