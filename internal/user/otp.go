package user

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OTPVerificationResult struct {
	UsedLegacyRecoveryCode bool
}

func VerifyOTP(user *model.User, otp, recoveryCode string) (result OTPVerificationResult, err error) {
	if otp != "" {
		decrypted, err := crypto.AesDecrypt(user.OTPSecret)
		if err != nil {
			return result, err
		}

		if ok := totp.Validate(otp, string(decrypted)); !ok {
			return result, ErrOTPCode
		}
	} else {
		// get user from db
		u := query.User
		user, err = u.Where(u.ID.Eq(user.ID)).First()
		if err != nil {
			return result, err
		}

		// legacy recovery code compatibility path
		if !user.RecoveryCodeGenerated() {
			err = model.UseDB().Transaction(func(tx *gorm.DB) error {
				var lockedUser model.User
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedUser, user.ID).Error; err != nil {
					return err
				}

				if lockedUser.OTPSecret == nil {
					return ErrTOTPNotEnabled
				}

				if lockedUser.RecoveryCodeGenerated() || lockedUser.RecoveryCodes.LegacyRecoveryCodeUsedAt != nil {
					return ErrRecoveryCode
				}

				recoverCode, err := hex.DecodeString(recoveryCode)
				if err != nil || len(recoverCode) != sha1.Size {
					return ErrRecoveryCode
				}

				k := sha1.Sum(lockedUser.OTPSecret)
				if !bytes.Equal(k[:], recoverCode) {
					return ErrRecoveryCode
				}

				t := time.Now().Unix()
				lockedUser.RecoveryCodes.LegacyRecoveryCodeUsedAt = &t
				if err := tx.Model(&lockedUser).Select("recovery_codes").Updates(&lockedUser).Error; err != nil {
					return err
				}

				result.UsedLegacyRecoveryCode = true
				return nil
			})
			return result, err
		}

		// check recovery code
		usedCount := 0
		verified := false
		for _, code := range user.RecoveryCodes.Codes {
			if !verified && code.Code == recoveryCode && code.UsedTime == nil {
				t := time.Now().Unix()
				code.UsedTime = &t
				_, err = u.Where(u.ID.Eq(user.ID)).Updates(user)
				if err != nil {
					return result, err
				}
				verified = true
			}
			if code.UsedTime != nil {
				usedCount++
			}
		}
		if !verified {
			return result, ErrRecoveryCode
		}
		if usedCount == len(user.RecoveryCodes.Codes) {
			notification.Warning("All Recovery Codes Have Been Used", "Please generate new recovery codes in the preferences immediately to prevent lockout.", nil)
		}
		return result, nil
	}
	return
}

func secureSessionIDCacheKey(sessionId string) string {
	return fmt.Sprintf("2fa_secure_session:_%s", sessionId)
}

// DefaultSecureSessionDuration is the two factor session window used by every
// release build. See secure_session.go and secure_session_dev.go.
const DefaultSecureSessionDuration = 10 * time.Minute

func SetSecureSessionID(userId uint64) (sessionId string) {
	sessionId = uuid.NewString()
	storeSecureSession(sessionId, userId, SecureSessionDuration())

	return
}

func VerifySecureSessionID(sessionId string, userId uint64) bool {
	storedUserID, ok := lookupSecureSession(sessionId)
	return ok && storedUserID == userId
}

// setCachedSecureSession and lookupCachedSecureSession back the release build
// and act as the fallback for the dev build before the database is ready.
func setCachedSecureSession(sessionId string, userId uint64, ttl time.Duration) {
	cache.Set(secureSessionIDCacheKey(sessionId), userId, ttl)
}

func lookupCachedSecureSession(sessionId string) (uint64, bool) {
	v, ok := cache.Get(secureSessionIDCacheKey(sessionId))
	if !ok {
		return 0, false
	}
	userId, ok := v.(uint64)
	return userId, ok
}
