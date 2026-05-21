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
			if user.OTPSecret == nil {
				return result, ErrTOTPNotEnabled
			}

			if user.RecoveryCodes.LegacyRecoveryCodeUsedAt != nil {
				return result, ErrRecoveryCode
			}

			recoverCode, err := hex.DecodeString(recoveryCode)
			if err != nil {
				return result, err
			}
			k := sha1.Sum(user.OTPSecret)
			if !bytes.Equal(k[:], recoverCode) {
				return result, ErrRecoveryCode
			}

			t := time.Now().Unix()
			user.RecoveryCodes.LegacyRecoveryCodeUsedAt = &t
			_, err = u.Where(u.ID.Eq(user.ID)).Updates(user)
			if err != nil {
				return result, err
			}

			result.UsedLegacyRecoveryCode = true
			return result, nil
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

func SetSecureSessionID(userId uint64) (sessionId string) {
	sessionId = uuid.NewString()
	cache.Set(secureSessionIDCacheKey(sessionId), userId, 10*time.Minute)

	return
}

func VerifySecureSessionID(sessionId string, userId uint64) bool {
	if v, ok := cache.Get(secureSessionIDCacheKey(sessionId)); ok {
		if v.(uint64) == userId {
			return true
		}
	}
	return false
}
