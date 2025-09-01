package llm

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"github.com/uozi-tech/cosy"
)

func StoreLLMRecord(c *gin.Context) {
	var json struct {
		FileName string                         `json:"file_name"`
		Messages []openai.ChatCompletionMessage `json:"messages"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	name := json.FileName
	g := query.LLMMessages
	_, err := g.Where(g.Name.Eq(name)).FirstOrCreate()

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	_, err = g.Where(g.Name.Eq(name)).Updates(&model.LLMMessages{
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
