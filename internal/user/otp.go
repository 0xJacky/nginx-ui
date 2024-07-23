package user

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/pkg/errors"
	"github.com/pquerna/otp/totp"
)

var (
	ErrOTPCode      = errors.New("invalid otp code")
	ErrRecoveryCode = errors.New("invalid recovery code")
)

func VerifyOTP(user *model.Auth, otp, recoveryCode string) (err error) {
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
