package config

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
)

func AddConfig(c *gin.Context) {
	var request struct {
		Name    string `json:"name" binding:"required"`
		Content string `json:"content" binding:"required"`
	}

	err := c.BindJSON(&request)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	name := request.Name
	content := request.Content

	path := nginx.GetConfPath("/", name)

	if _, err = os.Stat(path); err == nil {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "config exist",
		})
		return
	}

	if content != "" {
		err = os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}
	}

	output := nginx.Reload()
	if nginx.GetLogLevel(output) >= nginx.Warn {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output,
		})
		return
	}

	c.JSON(http.StatusOK, config.Config{
		Name:    name,
		Content: content,
	})
}
