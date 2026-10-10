package user

import (
	"fmt"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
)

type OTPVerificationResult struct {
	UsedLegacyRecoveryCode bool
}

func VerifyOTP(user *model.User, otp, recoveryCode string) (result OTPVerificationResult, err error) {
	if otp == "" {
		return verifyRecoveryCode(user, recoveryCode)
	}
	decrypted, err := crypto.AesDecrypt(user.OTPSecret)
	if err != nil {
		return result, err
	}
	if !totp.Validate(otp, string(decrypted)) {
		return result, ErrOTPCode
	}
	return result, nil
}

func secureSessionIDCacheKey(sessionId string) string {
	return fmt.Sprintf("2fa_secure_session:_%s", sessionId)
}

// DefaultSecureSessionDuration is the fallback two factor session window when
// no valid timeout is configured. See secure_session.go and secure_session_dev.go.
const DefaultSecureSessionDuration = time.Duration(settings.DefaultSecureSessionTimeoutMinutes) * time.Minute

const maxSecureSessionTimeoutMinutes = int((1<<63 - 1) / int64(time.Minute))

func configuredSecureSessionDuration() time.Duration {
	minutes := settings.AuthSettings.SecureSessionTimeoutMinutes
	if minutes <= 0 || minutes > maxSecureSessionTimeoutMinutes {
		return DefaultSecureSessionDuration
	}
	return time.Duration(minutes) * time.Minute
}

func SetSecureSessionID(userId uint64, versions ...uint64) (sessionId string) {
	sessionId = uuid.NewString()
	version, _ := secureSessionMFAVersion(userId)
	if len(versions) > 0 {
		version = versions[0]
	}
	storeSecureSession(sessionId, userId, version, SecureSessionDuration())

	return
}

func VerifySecureSessionID(sessionId string, userId uint64) bool {
	parsedSessionID, err := uuid.Parse(sessionId)
	if err != nil || parsedSessionID.String() != sessionId {
		return false
	}

	storedUserID, ok := lookupSecureSession(sessionId)
	if !ok || storedUserID != userId {
		return false
	}
	if model.UseDB() == nil {
		return true
	}
	version, err := secureSessionMFAVersion(userId)
	if err != nil {
		return false
	}
	storedVersion, _ := cache.Get(secureSessionIDCacheKey(sessionId) + ":version")
	if storedVersion == nil {
		return version == 0
	}
	return storedVersion == version
}

// setCachedSecureSession and lookupCachedSecureSession back the release build
// and act as the fallback for the dev build before the database is ready.
func setCachedSecureSession(sessionId string, userId uint64, ttl time.Duration, versions ...uint64) {
	cache.Set(secureSessionIDCacheKey(sessionId), userId, ttl)
	version, err := secureSessionMFAVersion(userId)
	if len(versions) > 0 {
		version, err = versions[0], nil
	}
	if err == nil {
		cache.Set(secureSessionIDCacheKey(sessionId)+":version", version, ttl)
	}
}

func lookupCachedSecureSession(sessionId string) (uint64, bool) {
	key := secureSessionIDCacheKey(sessionId)
	v, ok := cache.Get(key)
	if !ok {
		return 0, false
	}
	userId, ok := v.(uint64)
	if !ok {
		cache.Del(key)
	}
	return userId, ok
}
