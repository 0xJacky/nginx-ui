//go:build !dev

package user

import "time"

// SecureSessionDuration is how long a verified two-factor session is accepted.
func SecureSessionDuration() time.Duration {
	return configuredSecureSessionDuration()
}

// storeSecureSession keeps the session in memory only, so restarting the
// process invalidates every verified session.
func storeSecureSession(sessionId string, userId uint64, ttl time.Duration) {
	setCachedSecureSession(sessionId, userId, ttl)
}

func lookupSecureSession(sessionId string) (uint64, bool) {
	return lookupCachedSecureSession(sessionId)
}
