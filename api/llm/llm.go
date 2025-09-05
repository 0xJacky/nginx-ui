package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/llm"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)


func MakeChatCompletionRequest(c *gin.Context) {
	var json struct {
		Type        string                         `json:"type"`
		Messages    []openai.ChatCompletionMessage `json:"messages"`
		Language    string                         `json:"language,omitempty"`
		NginxConfig string                         `json:"nginx_config,omitempty"` // Separate field for nginx configuration content
		OSInfo      string                         `json:"os_info,omitempty"`      // Operating system information
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	// Choose appropriate system prompt based on the type
	var systemPrompt string
	if json.Type == "terminal" {
		systemPrompt = llm.TerminalAssistantPrompt
		
		// Add OS context for terminal assistant
		if json.OSInfo != "" {
			systemPrompt += fmt.Sprintf("\n\nSystem Information: %s", json.OSInfo)
		}
	} else {
		systemPrompt = llm.NginxConfigPrompt
	}

	// Append language instruction if language is provided
	if json.Language != "" {
		systemPrompt += fmt.Sprintf("\n\nIMPORTANT: Please respond in the language corresponding to this language code: %s", json.Language)
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}

	// Add nginx configuration context if provided
	if json.Type != "terminal" && json.NginxConfig != "" {
		// Add nginx configuration as context to the first user message
		if len(json.Messages) > 0 && json.Messages[0].Role == openai.ChatMessageRoleUser {
			// Prepend the nginx configuration to the first user message
			contextualContent := fmt.Sprintf("Nginx Configuration:\n```nginx\n%s\n```\n\n%s", json.NginxConfig, json.Messages[0].Content)
			json.Messages[0].Content = contextualContent
		}
	}

	messages = append(messages, json.Messages...)

	// SSE server
	api.SetSSEHeaders(c)

	openaiClient, err := llm.GetClient()
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
		messageCh := make(chan string)

		// 消息接收协程
		go func() {
			defer close(messageCh)
			for {
				response, err := stream.Recv()
				if errors.Is(err, io.EOF) {
					return
				}
				if err != nil {
					messageCh <- fmt.Sprintf("error: %v", err)
					logger.Errorf("Stream error: %v\n", err)
					return
				}
				messageCh <- response.Choices[0].Delta.Content
			}
		}()

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		var buffer strings.Builder

		for {
			select {
			case msg, ok := <-messageCh:
				if !ok {
					if buffer.Len() > 0 {
						msgChan <- buffer.String()
					}
					return
				}
				if strings.HasPrefix(msg, "error: ") {
					msgChan <- msg
					return
				}
				buffer.WriteString(msg)
			case <-ticker.C:
				if buffer.Len() > 0 {
					msgChan <- buffer.String()
					buffer.Reset()
				}
			}
		}
	}()

	c.Stream(func(w io.Writer) bool {
		m, ok := <-msgChan
		if !ok {
			return false
		}
		if strings.HasPrefix(m, "error: ") {
			c.SSEvent("message", gin.H{
				"type":    "error",
				"content": strings.TrimPrefix(m, "error: "),
			})
			return false
		}
		c.SSEvent("message", gin.H{
			"type":    "message",
			"content": m,
		})
		return true
	})
}
