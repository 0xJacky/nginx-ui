package config

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
)

type EditConfigJson struct {
	Content string `json:"content" binding:"required"`
}

func EditConfig(c *gin.Context) {
	name := c.Param("name")
	var request EditConfigJson
	err := c.BindJSON(&request)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	path := nginx.GetConfPath("/", name)
	content := request.Content

	origContent, err := os.ReadFile(path)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	if content != "" && content != string(origContent) {
		// model.CreateBackup(path)
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

	GetConfig(c)
}
