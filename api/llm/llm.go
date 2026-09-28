package llm

import (
	"context"
	"fmt"
	"io"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/llm"
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
		Model       string                         `json:"model,omitempty"`        // Empty for the default model
		Thinking    string                         `json:"thinking,omitempty"`     // Empty for the provider default
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

	// Earlier reasoning is shown in the chat but not sent back: providers
	// such as DeepSeek reject assistant messages that carry it.
	for i := range json.Messages {
		json.Messages[i].ReasoningContent = ""
	}

	messages = append(messages, json.Messages...)

	// SSE server
	api.SetSSEHeaders(c)

	modelName, extraBody, err := llm.ChatRequestOptions(json.Model, json.Thinking)
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

	ctx := llm.WithExtraBody(context.Background(), extraBody)

	req := openai.ChatCompletionRequest{
		Model:    modelName,
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

	c.Stream(relayChatStream(stream, func(event streamEvent) {
		c.SSEvent("message", gin.H{
			"type":    event.kind,
			"content": event.text,
		})
	}))
}
