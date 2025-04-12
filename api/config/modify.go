package config

import (
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"github.com/uozi-tech/cosy"
	"gorm.io/gen/field"
)

type EditConfigJson struct {
	Content string `json:"content" binding:"required"`
}

func EditConfig(c *gin.Context) {
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

	var json struct {
		Content       string   `json:"content"`
		SyncOverwrite bool     `json:"sync_overwrite"`
		SyncNodeIds   []uint64 `json:"sync_node_ids"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}

	absPath := nginx.GetConfPath(relativePath)
	if !helper.FileExists(absPath) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "file not found",
		})
		return
	}

	q := query.Config
	cfg, err := q.Assign(field.Attrs(&model.Config{
		Filepath: absPath,
	})).Where(q.Filepath.Eq(absPath)).FirstOrCreate()
	if err != nil {
		return
	}

	// Update database record
	_, err = q.Where(q.Filepath.Eq(absPath)).
		Select(q.SyncNodeIds, q.SyncOverwrite).
		Updates(&model.Config{
			SyncNodeIds:   json.SyncNodeIds,
			SyncOverwrite: json.SyncOverwrite,
		})
	if err != nil {
		return
	}

	cfg.SyncNodeIds = json.SyncNodeIds
	cfg.SyncOverwrite = json.SyncOverwrite

	content := json.Content
	err = config.Save(absPath, content, cfg)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	g := query.ChatGPTLog
	chatgpt, err := g.Where(g.Name.Eq(absPath)).FirstOrCreate()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if chatgpt.Content == nil {
		chatgpt.Content = make([]openai.ChatCompletionMessage, 0)
	}

	c.JSON(http.StatusOK, config.Config{
		Name:            filepath.Base(absPath),
		Content:         content,
		ChatGPTMessages: chatgpt.Content,
		FilePath:        absPath,
		ModifiedAt:      time.Now(),
		Dir:             filepath.Dir(relativePath),
	})
}
