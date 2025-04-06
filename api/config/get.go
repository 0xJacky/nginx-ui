package config

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"github.com/uozi-tech/cosy"
)

type APIConfigResp struct {
	config.Config
	SyncNodeIds   []uint64 `json:"sync_node_ids" gorm:"serializer:json"`
	SyncOverwrite bool     `json:"sync_overwrite"`
}

func GetConfig(c *gin.Context) {
	relativePath := c.Param("path")

	// Ensure the path is correctly decoded - handle cases where it might be encoded multiple times
	decodedPath := relativePath
	var err error
	// Try decoding until the path no longer changes
	for {
		newDecodedPath, decodeErr := url.PathUnescape(decodedPath)
		if decodeErr != nil || newDecodedPath == decodedPath {
			break
		}
		decodedPath = newDecodedPath
	}
	relativePath = decodedPath

	absPath := nginx.GetConfPath(relativePath)
	if !helper.IsUnderDirectory(absPath, nginx.GetConfPath()) {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "path is not under the nginx conf path",
		})
		return
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	q := query.Config
	g := query.ChatGPTLog
	chatgpt, err := g.Where(g.Name.Eq(absPath)).FirstOrCreate()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if chatgpt.Content == nil {
		chatgpt.Content = make([]openai.ChatCompletionMessage, 0)
	}

	cfg, err := q.Where(q.Filepath.Eq(absPath)).FirstOrInit()
	if err != nil {
		cosy.ErrHandler(c, err)
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
