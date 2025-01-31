package model

import (
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

type User struct {
	Model

	Name         string `json:"name" cosy:"add:max=20;update:omitempty,max=20;list:fussy;db_unique"`
	Password     string `json:"-" cosy:"add:required,max=20;update:omitempty,max=20"`
	Status       bool   `json:"status" gorm:"default:1"`
	OTPSecret    []byte `json:"-" gorm:"type:blob"`
	EnabledTwoFA bool   `json:"enabled_2fa" gorm:"-"`
}

type AuthToken struct {
	UserID    uint64 `json:"user_id"`
	Token     string `json:"token"`
	ExpiredAt int64  `json:"expired_at" gorm:"default:0"`
}

func (u *User) TableName() string {
	return "auths"
}

func (u *User) AfterFind(_ *gorm.DB) error {
	u.EnabledTwoFA = u.Enabled2FA()
	return nil
}

func (u *User) EnabledOTP() bool {
	return len(u.OTPSecret) != 0
}

func (u *User) EnabledPasskey() bool {
	var passkeys Passkey
	db.Where("user_id", u.ID).Limit(1).Find(&passkeys)
	return passkeys.ID != 0
}

func (u *User) Enabled2FA() bool {
	return u.EnabledOTP() || u.EnabledPasskey()
}

func (u *User) WebAuthnID() []byte {
	return []byte(cast.ToString(u.ID))
}

func (u *User) WebAuthnName() string {
	return u.Name
}

func (u *User) WebAuthnDisplayName() string {
	return u.Name
}

func (u *User) WebAuthnCredentials() (credentials []webauthn.Credential) {
	var passkeys []Passkey
	db.Where("user_id", u.ID).Find(&passkeys)
	for _, passkey := range passkeys {
		credentials = append(credentials, *passkey.Credential)
	}
	return
}
