package config

import (
	"time"

	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/sashabaranov/go-openai"
)

type ConfigStatus string

const (
	StatusEnabled     ConfigStatus = "enabled"
	StatusDisabled    ConfigStatus = "disabled"
	StatusMaintenance ConfigStatus = "maintenance"
)

// ProxyTarget is an alias for upstream.ProxyTarget
type ProxyTarget = upstream.ProxyTarget

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
	Status          ConfigStatus                   `json:"status"`
	Dir             string                         `json:"dir"`
	Urls            []string                       `json:"urls,omitempty"`
	ProxyTargets    []ProxyTarget                  `json:"proxy_targets,omitempty"`
	SyncNodeIds     []uint64                       `json:"sync_node_ids,omitempty"`
	SyncOverwrite   bool                           `json:"sync_overwrite"`
}
