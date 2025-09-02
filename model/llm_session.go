package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"gorm.io/gorm"
)

type LLMCompletionMessages []openai.ChatCompletionMessage

type LLMSession struct {
	ID           int                   `json:"id" gorm:"primaryKey"`
	SessionID    string                `json:"session_id" gorm:"uniqueIndex;not null"`
	Title        string                `json:"title"`
	Path         string                `json:"path" gorm:"index"` // 文件路径，可以为空
	Messages     LLMCompletionMessages `json:"messages" gorm:"serializer:json"`
	MessageCount int                   `json:"message_count"`
	IsActive     bool                  `json:"is_active" gorm:"default:true"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	DeletedAt    gorm.DeletedAt        `json:"-" gorm:"index"`
}

func (LLMSession) TableName() string {
	return "llm_sessions"
}

func (s *LLMSession) BeforeCreate(tx *gorm.DB) error {
	if s.SessionID == "" {
		s.SessionID = uuid.New().String()
	}
	return nil
}