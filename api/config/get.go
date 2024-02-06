package config

import (
    "github.com/0xJacky/Nginx-UI/api"
    "github.com/0xJacky/Nginx-UI/internal/config"
    "github.com/0xJacky/Nginx-UI/internal/nginx"
    "github.com/0xJacky/Nginx-UI/query"
    "github.com/gin-gonic/gin"
    "github.com/sashabaranov/go-openai"
    "net/http"
    "os"
)

func GetConfig(c *gin.Context) {
    name := c.Param("name")

    path := nginx.GetConfPath("/", name)

    stat, err := os.Stat(path)

    if err != nil {
        api.ErrHandler(c, err)
        return
    }

    content, err := os.ReadFile(path)

    if err != nil {
        api.ErrHandler(c, err)
        return
    }

    g := query.ChatGPTLog
    chatgpt, err := g.Where(g.Name.Eq(path)).FirstOrCreate()

    if err != nil {
        api.ErrHandler(c, err)
        return
    }

    if chatgpt.Content == nil {
        chatgpt.Content = make([]openai.ChatCompletionMessage, 0)
    }

    c.JSON(http.StatusOK, config.Config{
        Name:            name,
        Content:         string(content),
        ChatGPTMessages: chatgpt.Content,
        FilePath:        path,
        ModifiedAt:      stat.ModTime(),
    })
}
