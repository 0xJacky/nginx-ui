package config

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"net/http"
	"os"
	"path/filepath"
)

type APIConfigResp struct {
	config.Config
	SyncNodeIds   []uint64 `json:"sync_node_ids" gorm:"serializer:json"`
	SyncOverwrite bool     `json:"sync_overwrite"`
}

func GetConfig(c *gin.Context) {
	relativePath := c.Param("path")

	absPath := nginx.GetConfPath(relativePath)
	if !helper.IsUnderDirectory(absPath, nginx.GetConfPath()) {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "path is not under the nginx conf path",
		})
		return
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	q := query.Config
	g := query.ChatGPTLog
	chatgpt, err := g.Where(g.Name.Eq(absPath)).FirstOrCreate()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	if chatgpt.Content == nil {
		chatgpt.Content = make([]openai.ChatCompletionMessage, 0)
	}

	cfg, err := q.Where(q.Filepath.Eq(absPath)).FirstOrInit()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, APIConfigResp{
		Config: config.Config{
			Name:            stat.Name(),
			Content:         string(content),
			ChatGPTMessages: chatgpt.Content,
			FilePath:        absPath,
			ModifiedAt:      stat.ModTime(),
			Dir:             filepath.Dir(relativePath),
		},
		SyncNodeIds:   cfg.SyncNodeIds,
		SyncOverwrite: cfg.SyncOverwrite,
	})
}
