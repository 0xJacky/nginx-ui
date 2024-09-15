package user

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/pquerna/otp/totp"
	"time"
)

var (
	ErrOTPCode      = errors.New("invalid otp code")
	ErrRecoveryCode = errors.New("invalid recovery code")
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
		recoverCode, err := hex.DecodeString(recoveryCode)
		if err != nil {
			return err
		}
		k := sha1.Sum(user.OTPSecret)
		if !bytes.Equal(k[:], recoverCode) {
			return ErrRecoveryCode
		}
	}
	return
}

func secureSessionIDCacheKey(sessionId string) string {
	return fmt.Sprintf("otp_secure_session:_%s", sessionId)
}

func SetSecureSessionID(userId int) (sessionId string) {
	sessionId = uuid.NewString()
	cache.Set(secureSessionIDCacheKey(sessionId), userId, 5*time.Minute)

	return
}

func VerifySecureSessionID(sessionId string, userId int) bool {
	if v, ok := cache.Get(secureSessionIDCacheKey(sessionId)); ok {
		if v.(int) == userId {
			return true
		}
	}
	return false
}
