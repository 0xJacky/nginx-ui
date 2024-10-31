package config

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"net/http"
	"os"
	"time"
)

type EditConfigJson struct {
	Content string `json:"content" binding:"required"`
}

func EditConfig(c *gin.Context) {
	name := c.Param("name")
	var json struct {
		Name          string   `json:"name" binding:"required"`
		Filepath      string   `json:"filepath" binding:"required"`
		NewFilepath   string   `json:"new_filepath" binding:"required"`
		Content       string   `json:"content"`
		SyncOverwrite bool     `json:"sync_overwrite"`
		SyncNodeIds   []uint64 `json:"sync_node_ids"`
	}
	if !api.BindAndValid(c, &json) {
		return
	}

	path := json.Filepath
	if !helper.IsUnderDirectory(path, nginx.GetConfPath()) {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "filepath is not under the nginx conf path",
		})
		return
	}

	if !helper.IsUnderDirectory(json.NewFilepath, nginx.GetConfPath()) {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "new filepath is not under the nginx conf path",
		})
		return
	}

	if !helper.FileExists(path) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "file not found",
		})
		return
	}

	content := json.Content
	origContent, err := os.ReadFile(path)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	if content != "" && content != string(origContent) {
		err = os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}
	}

	q := query.Config
	cfg, err := q.Where(q.Filepath.Eq(json.Filepath)).FirstOrCreate()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	_, err = q.Where(q.Filepath.Eq(json.Filepath)).
		Select(q.Name, q.Filepath, q.SyncNodeIds, q.SyncOverwrite).
		Updates(&model.Config{
			Name:          json.Name,
			Filepath:      json.NewFilepath,
			SyncNodeIds:   json.SyncNodeIds,
			SyncOverwrite: json.SyncOverwrite,
		})
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	g := query.ChatGPTLog
	// handle rename
	if path != json.NewFilepath {
		if helper.FileExists(json.NewFilepath) {
			c.JSON(http.StatusNotAcceptable, gin.H{
				"message": "File exists",
			})
			return
		}
		err := os.Rename(json.Filepath, json.NewFilepath)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}

		// update ChatGPT record
		_, _ = g.Where(g.Name.Eq(json.NewFilepath)).Delete()
		_, _ = g.Where(g.Name.Eq(path)).Update(g.Name, json.NewFilepath)
	}

	err = config.SyncToRemoteServer(cfg, json.NewFilepath)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	output := nginx.Reload()
	if nginx.GetLogLevel(output) >= nginx.Warn {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output,
		})
		return
	}

	chatgpt, err := g.Where(g.Name.Eq(json.NewFilepath)).FirstOrCreate()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	if chatgpt.Content == nil {
		chatgpt.Content = make([]openai.ChatCompletionMessage, 0)
	}

	c.JSON(http.StatusOK, config.Config{
		Name:            name,
		Content:         content,
		ChatGPTMessages: chatgpt.Content,
		FilePath:        json.NewFilepath,
		ModifiedAt:      time.Now(),
	})
}
