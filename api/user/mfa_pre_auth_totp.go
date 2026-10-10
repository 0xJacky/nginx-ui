package user

import (
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm"
)

func BeginMFATOTP(c *gin.Context) {
	withMFAPreAuth(c, func(s *mfaPreAuth, u *model.User, _ string, stage string) {
		if stage == "setup" && !allowMFAEnrollment(c) {
			return
		}
		if stage != "setup" {
			cosy.ErrHandler(c, user.ErrMFAVerifyRequired)
			return
		}
		key, err := generateTOTPKey(u)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
		s.Secret = key.Secret()
		c.JSON(http.StatusOK, gin.H{"secret": key.Secret(), "url": key.URL()})
	})
}

func newMFARecoveryCodes() (model.RecoveryCodes, error) {
	codes, err := generateRecoveryCodes(16)
	t := time.Now().Unix()
	return model.RecoveryCodes{Codes: codes, LastViewed: &t}, err
}

func FinishMFATOTP(c *gin.Context) {
	withMFAPreAuth(c, func(s *mfaPreAuth, u *model.User, id, stage string) {
		if stage == "setup" && !allowMFAEnrollment(c) {
			return
		}
		if stage != "setup" || s.Secret == "" {
			cosy.ErrHandler(c, user.ErrMFAVerifyRequired)
			return
		}
		var payload struct {
			Passcode string `json:"passcode" binding:"required,len=6"`
		}
		if !cosy.BindAndValid(c, &payload) {
			return
		}
		s.Attempts++
		if !totp.Validate(payload.Passcode, s.Secret) {
			rejectMFAVerification(c, user.ErrOTPCode)
			return
		}
		secret, err := crypto.AesEncrypt([]byte(s.Secret))
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
		codes, err := newMFARecoveryCodes()
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
		u.OTPSecret, u.RecoveryCodes = secret, codes
		if err := user.SaveMFACredentials(u, func(tx *gorm.DB) error {
			return tx.Model(u).Select("otp_secret", "recovery_codes").Updates(u).Error
		}); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
		user.InvalidateUserCache(u.ID)
		completeMFAPreAuth(c, s, u, id, user.LoginProofOTP, &codes)
	})
}

func VerifyMFAPreAuthOTP(c *gin.Context) {
	withMFAPreAuth(c, func(s *mfaPreAuth, u *model.User, id, stage string) {
		if stage != "verify" {
			cosy.ErrHandler(c, user.ErrMFASetupRequired)
			return
		}
		var payload struct {
			OTP          string `json:"otp"`
			RecoveryCode string `json:"recovery_code"`
		}
		if !cosy.BindAndValid(c, &payload) {
			return
		}
		if payload.OTP == "" && payload.RecoveryCode == "" {
			cosy.ErrHandler(c, user.ErrOTPOrRecoveryCodeEmpty)
			return
		}
		s.Attempts++
		if _, err := user.VerifyOTP(u, payload.OTP, payload.RecoveryCode); err != nil {
			rejectMFAVerification(c, err)
			return
		}
		completeMFAPreAuth(c, s, u, id, user.LoginProofOTP, nil)
	})
}
