package openai

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"github.com/uozi-tech/cosy"
)

func StoreChatGPTRecord(c *gin.Context) {
	var json struct {
		FileName string                         `json:"file_name"`
		Messages []openai.ChatCompletionMessage `json:"messages"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	name := json.FileName
	g := query.ChatGPTLog
	_, err := g.Where(g.Name.Eq(name)).FirstOrCreate()

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	_, err = g.Where(g.Name.Eq(name)).Updates(&model.ChatGPTLog{
		Name:    name,
		Content: json.Messages,
	})

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
