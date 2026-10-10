package user

import (
	"errors"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/passkey"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var mfaLocks [64]sync.Mutex

// LockMFA serializes enrollment, removal and reset for one account.
func LockMFA(id uint64) func() {
	lock := &mfaLocks[id%uint64(len(mfaLocks))]
	lock.Lock()
	return lock.Unlock
}

func HasUsableMFA(u *model.User) (bool, error) {
	return hasUsableMFA(u, model.UseDB())
}

func hasUsableMFA(u *model.User, db *gorm.DB) (bool, error) {
	if u.EnabledOTP() {
		return true, nil
	}
	if !passkey.Enabled() {
		return false, nil
	}
	if u.ID == 0 {
		return false, nil
	}
	if db == nil {
		return false, errors.New("database is not initialized")
	}
	var count int64
	err := db.Model(&model.Passkey{}).Where("user_id = ?", u.ID).Count(&count).Error
	return count > 0, err
}

func LoginMFAStage(u *model.User, proof LoginProof) (string, error) {
	return loginMFAStage(u, proof, model.UseDB())
}

func loginMFAStage(u *model.User, proof LoginProof, db *gorm.DB) (string, error) {
	if proof == LoginProofSystem || proof == LoginProofOTP || proof == LoginProofPasskey {
		return "", nil
	}
	if proof == LoginProofExternal && !settings.AuthSettings.MFARequiredForSSO {
		return "", nil
	}
	enabled, err := hasUsableMFA(u, db)
	if err != nil {
		return "", err
	}
	if !enabled && u.MFAPolicy() != "optional" {
		return "setup", nil
	}
	if enabled && (proof == LoginProofExternal || u.MFAPolicy() != "optional") {
		return "verify", nil
	}
	return "", nil
}

func MFAVersion(id uint64) (uint64, error) {
	if model.UseDB() == nil {
		return 0, errors.New("database is not initialized")
	}
	var u struct{ MFAVersion uint64 }
	err := model.UseDB().Model(&model.User{}).Select("mfa_version").Where("id = ?", id).Scan(&u).Error
	return u.MFAVersion, err
}

// SaveMFACredentials rejects enrollment if a concurrent recovery reset occurred.
func SaveMFACredentials(u *model.User, change func(*gorm.DB) error) error {
	return model.UseDB().Transaction(func(tx *gorm.DB) error {
		var current model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "status", "mfa_version").First(&current, u.ID).Error; err != nil {
			return err
		}
		if !current.Status || current.MFAVersion != u.MFAVersion {
			return ErrSessionNotFound
		}
		return change(tx)
	})
}

// ChangeMFA checks the last-factor invariant in the same transaction as the write.
func ChangeMFA(id uint64, change func(*gorm.DB, *model.User) error, versions ...uint64) error {
	unlock := LockMFA(id)
	defer unlock()
	return model.UseDB().Transaction(func(tx *gorm.DB) error {
		var u model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, id).Error; err != nil {
			return err
		}
		if !u.Status || (len(versions) > 0 && u.MFAVersion != versions[0]) {
			return ErrSessionNotFound
		}
		if err := change(tx, &u); err != nil {
			return err
		}
		if u.EnabledOTP() {
			return nil
		}
		var count int64
		if err := tx.Model(&model.Passkey{}).Where("user_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if u.MFAPolicy() != "optional" && (!passkey.Enabled() || count == 0) {
			return ErrLastMFAFactor
		}
		if count == 0 {
			u.RecoveryCodes = model.RecoveryCodes{}
			return tx.Model(u).Select("recovery_codes").Updates(u).Error
		}
		return nil
	})
}
