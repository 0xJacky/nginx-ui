package config

import (
	"github.com/sashabaranov/go-openai"
	"time"
)

type Config struct {
	Name            string                         `json:"name"`
	Content         string                         `json:"content,omitempty"`
	ChatGPTMessages []openai.ChatCompletionMessage `json:"chatgpt_messages,omitempty"`
	FilePath        string                         `json:"file_path,omitempty"`
	ModifiedAt      time.Time                      `json:"modified_at"`
	Size            int64                          `json:"size,omitempty"`
	IsDir           bool                           `json:"is_dir"`
	Enabled         bool                           `json:"enabled"`
}
