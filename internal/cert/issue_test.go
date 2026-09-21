package cert

import (
	"os"
	"testing"
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

func TestInitChallengeEnvDisablesCNAMEFollowing(t *testing.T) {
	previous, existed := os.LookupEnv(envDisableCNAMESupport)
	t.Cleanup(func() {
		if existed {
			os.Setenv(envDisableCNAMESupport, previous)
			return
		}
		os.Unsetenv(envDisableCNAMESupport)
	})
	os.Unsetenv(envDisableCNAMESupport)

	InitChallengeEnv()

	if got := os.Getenv(envDisableCNAMESupport); got != "true" {
		t.Fatalf("%s = %q, want true", envDisableCNAMESupport, got)
	}
}
