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
