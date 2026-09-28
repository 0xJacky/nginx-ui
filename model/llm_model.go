package model

import "time"

// LLMThinkingParams maps a thinking level to the fields merged into the chat
// completion request body to select it, e.g. "high" to
// {"reasoning_effort": "high"}. A level without an entry is not offered.
type LLMThinkingParams map[string]map[string]any

// LLMModel is a model offered in the LLM assistant, together with the request
// fields that select each thinking level on its provider.
type LLMModel struct {
	ID             uint64            `gorm:"primary_key" json:"id"`
	Name           string            `json:"name" gorm:"uniqueIndex;not null"`
	SortOrder      int               `json:"sort_order"`
	ThinkingPreset string            `json:"thinking_preset"`
	ThinkingParams LLMThinkingParams `json:"thinking_params" gorm:"serializer:json"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

func (LLMModel) TableName() string {
	return "llm_models"
}
