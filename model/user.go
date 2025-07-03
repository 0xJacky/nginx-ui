package model

import (
	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/spf13/cast"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func init() {
	schema.RegisterSerializer("json[aes]", crypto.JSONAesSerializer{})
}

type RecoveryCode struct {
	Code     string `json:"code"`
	UsedTime *int64 `json:"used_time,omitempty"  gorm:"type:datetime;default:null"`
}

type RecoveryCodes struct {
	Codes          []*RecoveryCode `json:"codes"`
	LastViewed     *int64          `json:"last_viewed,omitempty" gorm:"serializer:unixtime;type:datetime;default:null"`
	LastDownloaded *int64          `json:"last_downloaded,omitempty" gorm:"serializer:unixtime;type:datetime;default:null"`
}

type User struct {
	Model

	Name          string        `json:"name" cosy:"add:max=20;update:omitempty,max=20;list:fussy;db_unique"`
	Password      string        `json:"-" cosy:"json:password;add:required,max=20;update:omitempty,max=20"`
	Status        bool          `json:"status" gorm:"default:1"`
	OTPSecret     []byte        `json:"-" gorm:"type:blob"`
	RecoveryCodes RecoveryCodes `json:"-" gorm:"serializer:json[aes]"`
	EnabledTwoFA  bool          `json:"enabled_2fa" gorm:"-"`
	Language      string        `json:"language" gorm:"default:en"`
}

type AuthToken struct {
	UserID     uint64 `json:"user_id"`
	Token      string `json:"token"`
	ShortToken string `json:"short_token"`
	ExpiredAt  int64  `json:"expired_at" gorm:"default:0"`
}

func (u *User) TableName() string {
	return "users"
}

func (u *User) AfterFind(_ *gorm.DB) error {
	u.EnabledTwoFA = u.Enabled2FA()
	return nil
}

func (u *User) EnabledOTP() bool {
	return len(u.OTPSecret) != 0
}

func (u *User) RecoveryCodeGenerated() bool {
	return len(u.RecoveryCodes.Codes) > 0
}

func (u *User) RecoveryCodeViewed() bool {
	return u.RecoveryCodes.LastViewed != nil
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
