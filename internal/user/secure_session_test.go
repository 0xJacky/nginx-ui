//go:build !dev

package user

import (
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestSecureSessionDurationIgnoresEnvInReleaseBuilds(t *testing.T) {
	originalTimeout := settings.AuthSettings.SecureSessionTimeoutMinutes
	t.Cleanup(func() {
		settings.AuthSettings.SecureSessionTimeoutMinutes = originalTimeout
	})
	settings.AuthSettings.SecureSessionTimeoutMinutes = 10
	t.Setenv("NGINX_UI_DEV_SECURE_SESSION_MINUTES", "600")

	if got := SecureSessionDuration(); got != DefaultSecureSessionDuration {
		t.Fatalf("duration = %s, want the fixed %s", got, DefaultSecureSessionDuration)
	}
}

func TestSecureSessionDurationUsesValidatedAuthSetting(t *testing.T) {
	originalTimeout := settings.AuthSettings.SecureSessionTimeoutMinutes
	t.Cleanup(func() {
		settings.AuthSettings.SecureSessionTimeoutMinutes = originalTimeout
	})

	settings.AuthSettings.SecureSessionTimeoutMinutes = 60
	if got := SecureSessionDuration(); got != time.Hour {
		t.Fatalf("duration = %s, want 1h from auth setting", got)
	}

	for _, invalid := range []int{0, -1} {
		settings.AuthSettings.SecureSessionTimeoutMinutes = invalid
		if got := SecureSessionDuration(); got != DefaultSecureSessionDuration {
			t.Fatalf("duration for invalid value %d = %s, want %s", invalid, got, DefaultSecureSessionDuration)
		}
	}
}
