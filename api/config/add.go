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
	"path/filepath"
	"time"
)

func AddConfig(c *gin.Context) {
	var json struct {
		Name        string   `json:"name" binding:"required"`
		NewFilepath string   `json:"new_filepath" binding:"required"`
		Content     string   `json:"content"`
		Overwrite   bool     `json:"overwrite"`
		SyncNodeIds []uint64 `json:"sync_node_ids"`
	}

	if !api.BindAndValid(c, &json) {
		return
	}

	name := json.Name
	content := json.Content
	path := json.NewFilepath
	if !helper.IsUnderDirectory(path, nginx.GetConfPath()) {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "new filepath is not under the nginx conf path",
		})
		return
	}

	if !json.Overwrite && helper.FileExists(path) {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "File exists",
		})
		return
	}

	// check if the dir exists, if not, use mkdirAll to create the dir
	dir := filepath.Dir(path)
	if !helper.FileExists(dir) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}
	}

	err := os.WriteFile(path, []byte(content), 0644)
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

	q := query.Config
	_, err = q.Where(q.Filepath.Eq(path)).Delete()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	err = q.Create(&model.Config{
		Name:          name,
		Filepath:      path,
		SyncNodeIds:   json.SyncNodeIds,
		SyncOverwrite: json.Overwrite,
	})
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, config.Config{
		Name:            name,
		Content:         content,
		ChatGPTMessages: make([]openai.ChatCompletionMessage, 0),
		FilePath:        path,
		ModifiedAt:      time.Now(),
	})
}
