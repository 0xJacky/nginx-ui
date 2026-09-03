//go:build dev

package user

import (
	"testing"
	"time"
)

func TestSecureSessionDurationDevOverride(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want time.Duration
	}{
		{name: "unset falls back to the default", env: "", want: DefaultSecureSessionDuration},
		{name: "override is honoured", env: "120", want: 2 * time.Hour},
		{name: "value above the cap is clamped", env: "10000", want: MaxDevSecureSessionDuration},
		{name: "zero falls back to the default", env: "0", want: DefaultSecureSessionDuration},
		{name: "negative falls back to the default", env: "-5", want: DefaultSecureSessionDuration},
		{name: "garbage falls back to the default", env: "abc", want: DefaultSecureSessionDuration},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(SecureSessionDurationEnv, tc.env)
			if got := SecureSessionDuration(); got != tc.want {
				t.Fatalf("duration = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestSecureSessionDurationDevRejectsOverflowingValue(t *testing.T) {
	t.Setenv(SecureSessionDurationEnv, "999999999999")
	got := SecureSessionDuration()
	if got != MaxDevSecureSessionDuration {
		t.Fatalf("duration = %s, want the %s cap", got, MaxDevSecureSessionDuration)
	}
	if got <= 0 {
		t.Fatalf("duration must stay positive, got %s", got)
	}
}
