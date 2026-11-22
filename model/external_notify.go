package model

type ExternalNotify struct {
	Model
	Type        string            `json:"type" cosy:"add:required;update:omitempty" gorm:"index"`
	Description string            `json:"description" cosy:"add:omitempty;update:omitempty"`
	Language    string            `json:"language" cosy:"add:required;update:omitempty" gorm:"index"`
	Config      map[string]string `json:"config" cosy:"add:required;update:omitempty" gorm:"serializer:json[aes]"`
	Levels      []string          `json:"levels" cosy:"add:omitempty,dive,oneof=info success warning error;update:omitempty,dive,oneof=info success warning error" gorm:"serializer:json"`
	Enabled     bool              `json:"enabled" cosy:"add:omitempty;update:omitempty" gorm:"default:1"`
}
