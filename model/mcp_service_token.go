package model

import "time"

const (
	MCPTokenScopeRead  = "mcp:read"
	MCPTokenScopeWrite = "mcp:write"
)

type MCPServiceToken struct {
	Model
	PublicID   string     `json:"id" gorm:"uniqueIndex;not null"`
	Name       string     `json:"name" gorm:"not null"`
	Verifier   []byte     `json:"-" gorm:"not null"`
	Scopes     []string   `json:"scopes" gorm:"serializer:json;not null"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty" gorm:"index"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty" gorm:"index"`
	CreatorID  uint64     `json:"creator_id" gorm:"index;not null"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}
