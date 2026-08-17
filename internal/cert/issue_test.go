package cert

import "testing"

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

func TestDNS01ChallengeOptions(t *testing.T) {
	if got := len(dns01ChallengeOptions(&ConfigPayload{})); got != 1 {
		t.Fatalf("default option count = %d, want 1", got)
	}

	payload := &ConfigPayload{DisableAuthoritativeNSPropagation: true}
	if got := len(dns01ChallengeOptions(payload)); got != 2 {
		t.Fatalf("disabled authoritative propagation option count = %d, want 2", got)
	}
}
