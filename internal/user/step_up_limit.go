package user

import (
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
)

// stepUpWindow counts the failed step-up verifications of one account in a
// window that starts with the first failure.
type stepUpWindow struct {
	attempts  int
	expiresAt time.Time
}

// stepUpLimiter caps OTP and recovery code guesses for sessions that are
// already logged in. The login ban is keyed by client IP, which does not
// help here: a stolen session token can guess from any address, so the
// count belongs to the account. It follows the login ban settings
// (MaxAttempts per BanThresholdMinutes) and lives in memory, like the secure
// sessions it guards.
type stepUpLimiter struct {
	mu      sync.Mutex
	windows map[uint64]stepUpWindow
	now     func() time.Time
}

var stepUpAttempts = &stepUpLimiter{windows: map[uint64]stepUpWindow{}, now: time.Now}

// locked reports whether userID has used up its attempts in the current
// window. An ended window is forgotten, so a lock always lifts on its own.
func (l *stepUpLimiter) locked(userID uint64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.windows[userID]
	if !ok {
		return false
	}
	if !l.now().Before(w.expiresAt) {
		delete(l.windows, userID)
		return false
	}
	return w.attempts >= settings.AuthSettings.MaxAttempts
}

func (l *stepUpLimiter) fail(userID uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	w, ok := l.windows[userID]
	if !ok || !now.Before(w.expiresAt) {
		w = stepUpWindow{expiresAt: now.Add(time.Duration(settings.AuthSettings.BanThresholdMinutes) * time.Minute)}
	}
	w.attempts++
	l.windows[userID] = w
}

func (l *stepUpLimiter) reset(userID uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.windows, userID)
}

// VerifyStepUpOTP checks an OTP or recovery code to open a secure session for
// an account that is already logged in. Once the account has failed
// MaxAttempts times within BanThresholdMinutes it returns ErrTooManyAttempts
// without looking at the code, until the window ends. A success clears the
// count, so a few typos never add up across sessions.
func VerifyStepUpOTP(u *model.User, otp, recoveryCode string) (OTPVerificationResult, error) {
	// Hold the account's MFA lock across check, verification and count so
	// parallel requests cannot all pass the check before any failure is
	// recorded.
	unlock := LockMFA(u.ID)
	defer unlock()

	if stepUpAttempts.locked(u.ID) {
		return OTPVerificationResult{}, ErrTooManyAttempts
	}
	result, err := VerifyOTP(u, otp, recoveryCode)
	if err != nil {
		stepUpAttempts.fail(u.ID)
		return result, err
	}
	stepUpAttempts.reset(u.ID)
	return result, nil
}
