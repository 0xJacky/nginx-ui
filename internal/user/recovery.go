package user

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// verifyRecoveryCode consumes a code without writing stale account credentials.
func verifyRecoveryCode(account *model.User, code string) (result OTPVerificationResult, err error) {
	exhausted := false
	err = model.UseDB().Transaction(func(tx *gorm.DB) error {
		var current model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, account.ID).Error; err != nil {
			return err
		}
		if !current.Status || current.MFAVersion != account.MFAVersion {
			return ErrSessionNotFound
		}
		if !current.RecoveryCodeGenerated() {
			if !current.EnabledOTP() {
				return ErrTOTPNotEnabled
			}
			if current.RecoveryCodes.LegacyRecoveryCodeUsedAt != nil {
				return ErrRecoveryCode
			}
			decoded, err := hex.DecodeString(code)
			if err != nil || len(decoded) != sha1.Size {
				return ErrRecoveryCode
			}
			expected := sha1.Sum(current.OTPSecret)
			if !bytes.Equal(expected[:], decoded) {
				return ErrRecoveryCode
			}
			now := time.Now().Unix()
			current.RecoveryCodes.LegacyRecoveryCodeUsedAt = &now
			result.UsedLegacyRecoveryCode = true
		} else {
			verified, used := false, 0
			for _, candidate := range current.RecoveryCodes.Codes {
				if !verified && candidate.Code == code && candidate.UsedTime == nil {
					now := time.Now().Unix()
					candidate.UsedTime = &now
					verified = true
				}
				if candidate.UsedTime != nil {
					used++
				}
			}
			if !verified {
				return ErrRecoveryCode
			}
			exhausted = used == len(current.RecoveryCodes.Codes)
		}
		return tx.Model(&current).Select("recovery_codes").Updates(&current).Error
	})
	if err == nil && exhausted {
		notification.Warning("All Recovery Codes Have Been Used", "Please generate new recovery codes in the preferences immediately to prevent lockout.", nil)
	}
	return result, err
}
