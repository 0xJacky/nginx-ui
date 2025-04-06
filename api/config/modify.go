package config

import (
	"net/http"
	"os"
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

	content := json.Content
	origContent, err := os.ReadFile(absPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	err = config.CheckAndCreateHistory(absPath, content)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if content != "" && content != string(origContent) {
		err = os.WriteFile(absPath, []byte(content), 0644)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	q := query.Config
	cfg, err := q.Assign(field.Attrs(&model.Config{
		Name: filepath.Base(absPath),
	})).Where(q.Filepath.Eq(absPath)).FirstOrCreate()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	_, err = q.Where(q.Filepath.Eq(absPath)).
		Select(q.SyncNodeIds, q.SyncOverwrite).
		Updates(&model.Config{
			SyncNodeIds:   json.SyncNodeIds,
			SyncOverwrite: json.SyncOverwrite,
		})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// use the new values
	cfg.SyncNodeIds = json.SyncNodeIds
	cfg.SyncOverwrite = json.SyncOverwrite

	g := query.ChatGPTLog
	err = config.SyncToRemoteServer(cfg)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	output := nginx.Reload()
	if nginx.GetLogLevel(output) >= nginx.Warn {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output,
		})
		return
	}

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
