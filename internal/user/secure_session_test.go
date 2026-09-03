//go:build !dev

package user

import (
	"testing"
)

func TestSecureSessionDurationIgnoresEnvInReleaseBuilds(t *testing.T) {
	t.Setenv("NGINX_UI_DEV_SECURE_SESSION_MINUTES", "600")

	if got := SecureSessionDuration(); got != DefaultSecureSessionDuration {
		t.Fatalf("duration = %s, want the fixed %s", got, DefaultSecureSessionDuration)
	}
}
