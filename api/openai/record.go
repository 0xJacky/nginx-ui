package openai

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

func GetChatGPTRecord(c *gin.Context) {
	absPath := c.Query("path")

	if !helper.IsUnderDirectory(absPath, nginx.GetConfPath()) {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "path is not under the nginx conf path",
		})
		return
	}

	g := query.ChatGPTLog
	chatgpt, err := g.Where(g.Name.Eq(absPath)).FirstOrCreate()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, chatgpt)
}
