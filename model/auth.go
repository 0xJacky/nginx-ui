package model

type Auth struct {
	Model

	Name      string `json:"name"`
	Password  string `json:"-"`
	Status    bool   `json:"status" gorm:"default:1"`
	OTPSecret []byte `json:"-" gorm:"type:blob"`
}

type AuthToken struct {
	UserID    int    `json:"user_id"`
	Token     string `json:"token"`
	ExpiredAt int64  `json:"expired_at" gorm:"default:0"`
}

func (u *Auth) EnabledOTP() bool {
	return len(u.OTPSecret) != 0
}
