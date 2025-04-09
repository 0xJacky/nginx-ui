package model

type ExternalNotify struct {
	Model
	Type     string            `json:"type" cosy:"add:required;update:omitempty" gorm:"index"`
	Language string            `json:"language" cosy:"add:required;update:omitempty" gorm:"index"`
	Config   map[string]string `json:"config" cosy:"add:required;update:omitempty" gorm:"serializer:json[aes]"`
}
