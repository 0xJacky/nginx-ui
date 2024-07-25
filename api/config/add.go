package config

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"net/http"
	"os"
	"time"
)

func AddConfig(c *gin.Context) {
	var json struct {
		Name        string `json:"name" binding:"required"`
		NewFilepath string `json:"new_filepath" binding:"required"`
		Content     string `json:"content"`
		Overwrite   bool   `json:"overwrite"`
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

	c.JSON(http.StatusOK, config.Config{
		Name:            name,
		Content:         content,
		ChatGPTMessages: make([]openai.ChatCompletionMessage, 0),
		FilePath:        path,
		ModifiedAt:      time.Now(),
	})
}
