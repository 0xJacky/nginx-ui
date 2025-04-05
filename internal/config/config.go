package config

import (
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/sashabaranov/go-openai"
)

type Config struct {
	Name            string                         `json:"name"`
	Content         string                         `json:"content"`
	ChatGPTMessages []openai.ChatCompletionMessage `json:"chatgpt_messages,omitempty"`
	FilePath        string                         `json:"filepath,omitempty"`
	ModifiedAt      time.Time                      `json:"modified_at"`
	Size            int64                          `json:"size,omitempty"`
	IsDir           bool                           `json:"is_dir"`
	EnvGroupID      uint64                         `json:"env_group_id"`
	EnvGroup        *model.EnvGroup                `json:"env_group,omitempty"`
	Enabled         bool                           `json:"enabled"`
	Dir             string                         `json:"dir"`
}
