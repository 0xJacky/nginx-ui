package model

import "github.com/go-webauthn/webauthn/webauthn"

type Passkey struct {
	Model

	Name       string               `json:"name"`
	UserID     int                  `json:"user_id"`
	RawID      string               `json:"raw_id"`
	Credential *webauthn.Credential `json:"-" gorm:"serializer:json"`
	LastUsedAt int64                `json:"last_used_at" gorm:"default:0"`
}
