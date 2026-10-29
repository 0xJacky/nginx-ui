package user

import (
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
)

func newTestStepUpLimiter(clock *time.Time) *stepUpLimiter {
	return &stepUpLimiter{windows: map[uint64]stepUpWindow{}, now: func() time.Time { return *clock }}
}

func TestStepUpLimiterLocksAfterMaxAttemptsAndLiftsAfterThreshold(t *testing.T) {
	clock := time.Unix(1_800_000_000, 0)
	l := newTestStepUpLimiter(&clock)
	window := time.Duration(settings.AuthSettings.BanThresholdMinutes) * time.Minute

	for range settings.AuthSettings.MaxAttempts - 1 {
		l.fail(7)
	}
	require.False(t, l.locked(7), "failures below the limit must not lock")
	l.fail(7)
	require.True(t, l.locked(7))
	require.False(t, l.locked(8), "the lock is per account")

	clock = clock.Add(window - time.Second)
	require.True(t, l.locked(7), "the lock holds for the whole window")

	clock = clock.Add(time.Second)
	require.False(t, l.locked(7), "the lock lifts once BanThresholdMinutes have passed")

	// The next failure starts a new window from one instead of resuming.
	l.fail(7)
	require.False(t, l.locked(7))
	require.Equal(t, 1, l.windows[7].attempts)
}

func TestStepUpLimiterFailureAfterEndedWindowStartsOver(t *testing.T) {
	clock := time.Unix(1_800_000_000, 0)
	l := newTestStepUpLimiter(&clock)

	for range settings.AuthSettings.MaxAttempts - 1 {
		l.fail(7)
	}
	clock = clock.Add(time.Duration(settings.AuthSettings.BanThresholdMinutes) * time.Minute)
	l.fail(7)
	require.False(t, l.locked(7), "failures from an ended window must not carry over")
}

func TestStepUpLimiterResetClearsCount(t *testing.T) {
	clock := time.Unix(1_800_000_000, 0)
	l := newTestStepUpLimiter(&clock)

	for range settings.AuthSettings.MaxAttempts - 1 {
		l.fail(7)
	}
	l.reset(7)
	for range settings.AuthSettings.MaxAttempts - 1 {
		l.fail(7)
	}
	require.False(t, l.locked(7))
}
