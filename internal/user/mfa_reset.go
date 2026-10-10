package user

import (
	"github.com/0xJacky/Nginx-UI/model"
	"gorm.io/gorm"
)

// ResetMFA preserves policy while invalidating every credential and login.
func ResetMFA(id uint64) error {
	unlock := LockMFA(id)
	defer unlock()
	var tokens []model.AuthToken
	err := model.UseDB().Transaction(func(tx *gorm.DB) error {
		var u model.User
		if err := tx.First(&u, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
			"otp_secret": nil, "recovery_codes": nil,
			"mfa_version": gorm.Expr("mfa_version + 1"),
		}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&model.Passkey{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Find(&tokens).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", id).Delete(&model.AuthToken{}).Error
	})
	if err != nil {
		return err
	}
	InvalidateUserCache(id)
	for _, token := range tokens {
		InvalidateTokenCache(token.Token)
		InvalidateShortTokenCache(token.ShortToken)
	}
	CloseMFAConnections(id)
	return nil
}
