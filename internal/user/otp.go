package user

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
)

func VerifyOTP(user *model.User, otp, recoveryCode string) (err error) {
	if otp != "" {
		decrypted, err := crypto.AesDecrypt(user.OTPSecret)
		if err != nil {
			return err
		}

		if ok := totp.Validate(otp, string(decrypted)); !ok {
			return ErrOTPCode
		}
	} else {
		// get user from db
		u := query.User
		user, err = u.Where(u.ID.Eq(user.ID)).First()
		if err != nil {
			return err
		}

		// legacy recovery code
		if !user.RecoveryCodeGenerated() {
			if user.OTPSecret == nil {
				return ErrTOTPNotEnabled
			}

			recoverCode, err := hex.DecodeString(recoveryCode)
			if err != nil {
				return err
			}
			k := sha1.Sum(user.OTPSecret)
			if !bytes.Equal(k[:], recoverCode) {
				return ErrRecoveryCode
			}
		}

		// check recovery code
		for _, code := range user.RecoveryCodes.Codes {
			if code.Code == recoveryCode && code.UsedTime == nil {
				t := time.Now()
				code.UsedTime = &t
				_, err = u.Where(u.ID.Eq(user.ID)).Updates(user)
				return
			}
		}
		return ErrRecoveryCode
	}
	return
}

func secureSessionIDCacheKey(sessionId string) string {
	return fmt.Sprintf("2fa_secure_session:_%s", sessionId)
}

func SetSecureSessionID(userId uint64) (sessionId string) {
	sessionId = uuid.NewString()
	cache.Set(secureSessionIDCacheKey(sessionId), userId, 5*time.Minute)

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
