package openai

import (
	"context"
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/chatbot"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"errors"
	"github.com/sashabaranov/go-openai"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"io"
)

const ChatGPTInitPrompt = `You are a assistant who can help users write and optimise the configurations of Nginx,
the first user message contains the content of the configuration file which is currently opened by the user and
the current language code(CLC). You suppose to use the language corresponding to the CLC to give the first reply.
Later the language environment depends on the user message.
The first reply should involve the key information of the file and ask user what can you help them.`

func MakeChatCompletionRequest(c *gin.Context) {
	var json struct {
		Filepath string                         `json:"filepath"`
		Messages []openai.ChatCompletionMessage `json:"messages"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: ChatGPTInitPrompt,
		},
	}

	messages = append(messages, json.Messages...)

	if json.Filepath != "" {
		messages = chatbot.ChatCompletionWithContext(json.Filepath, messages)
	}

	// SSE server
	c.Writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	openaiClient, err := chatbot.GetClient()
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

	ctx := context.Background()

	req := openai.ChatCompletionRequest{
		Model:    settings.OpenAISettings.Model,
		Messages: messages,
		Stream:   true,
	}
	stream, err := openaiClient.CreateChatCompletionStream(ctx, req)
	if err != nil {
		logger.Errorf("CompletionStream error: %v\n", err)
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
		defer close(msgChan)
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}

			if err != nil {
				logger.Errorf("Stream error: %v\n", err)
				return
			}

			message := fmt.Sprintf("%s", response.Choices[0].Delta.Content)

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
