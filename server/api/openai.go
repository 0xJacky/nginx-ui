package api

import (
	"context"
	"fmt"
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/query"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/sashabaranov/go-openai"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
)

const ChatGPTInitPrompt = "You are a assistant who can help users write and optimise the configurations of Nginx, the first user message contains the content of the configuration file which is currently opened by the user and the current language code(CLC). You suppose to use the language corresponding to the CLC to give the first reply. Later the language environment depends on the user message. The first reply should involve the key information of the file and ask user what can you help them."

func MakeChatCompletionRequest(c *gin.Context) {
	var json struct {
		Messages []openai.ChatCompletionMessage `json:"messages"`
	}

	if !BindAndValid(c, &json) {
		return
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: ChatGPTInitPrompt,
		},
	}
	messages = append(messages, json.Messages...)
	// sse server
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	log.Println(settings.OpenAISettings.Token)

	config := openai.DefaultConfig(settings.OpenAISettings.Token)

	if settings.OpenAISettings.Proxy != "" {
		proxyUrl, err := url.Parse(settings.OpenAISettings.Proxy)
		if err != nil {
			c.Stream(func(w io.Writer) bool {
				c.SSEvent("message", gin.H{
					"type":    "error",
					"content": err.Error(),
				})
				return false
			})
			return
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
		config.HTTPClient = &http.Client{
			Transport: transport,
		}
	}

	if settings.OpenAISettings.BaseUrl != "" {
		config.BaseURL = settings.OpenAISettings.BaseUrl
	}

	openaiClient := openai.NewClientWithConfig(config)
	ctx := context.Background()

	req := openai.ChatCompletionRequest{
		Model:    settings.OpenAISettings.Model,
		Messages: messages,
		Stream:   true,
	}
	stream, err := openaiClient.CreateChatCompletionStream(ctx, req)
	if err != nil {
		fmt.Printf("CompletionStream error: %v\n", err)
		c.Stream(func(w io.Writer) bool {
			c.SSEvent("message", gin.H{
				"type":    "error",
				"content": err.Error(),
			})
			return false
		})
		return
	}
	defer stream.Close()
	msgChan := make(chan string)
	go func() {
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				close(msgChan)
				fmt.Println()
				return
			}

			if err != nil {
				fmt.Printf("Stream error: %v\n", err)
				close(msgChan)
				return
			}

			// Send SSE to client
			message := fmt.Sprintf("%s", response.Choices[0].Delta.Content)
			fmt.Printf("%s", response.Choices[0].Delta.Content)
			_ = os.Stdout.Sync()

			msgChan <- message
		}
	}()

	c.Stream(func(w io.Writer) bool {
		if m, ok := <-msgChan; ok {
			c.SSEvent("message", gin.H{
				"type":    "message",
				"content": m,
			})
			return true
		}
		return false
	})
}

func StoreChatGPTRecord(c *gin.Context) {
	var json struct {
		FileName string                         `json:"file_name"`
		Messages []openai.ChatCompletionMessage `json:"messages"`
	}

	if !BindAndValid(c, &json) {
		return
	}

	name := json.FileName
	g := query.ChatGPTLog
	_, err := g.Where(g.Name.Eq(name)).FirstOrCreate()

	if err != nil {
		ErrHandler(c, err)
		return
	}

	_, err = g.Where(g.Name.Eq(name)).Updates(&model.ChatGPTLog{
		Name:    name,
		Content: json.Messages,
	})

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
