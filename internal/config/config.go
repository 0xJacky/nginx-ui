package config

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/sashabaranov/go-openai"
	"time"
)

type Config struct {
	Name            string                         `json:"name"`
	Content         string                         `json:"content"`
	ChatGPTMessages []openai.ChatCompletionMessage `json:"chatgpt_messages,omitempty"`
	FilePath        string                         `json:"filepath,omitempty"`
	ModifiedAt      time.Time                      `json:"modified_at"`
	Size            int64                          `json:"size,omitempty"`
	IsDir           bool                           `json:"is_dir"`
	SiteCategoryID  uint64                         `json:"site_category_id"`
	SiteCategory    *model.SiteCategory            `json:"site_category,omitempty"`
	Enabled         bool                           `json:"enabled"`
}
