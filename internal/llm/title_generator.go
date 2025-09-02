package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/sashabaranov/go-openai"
	"github.com/uozi-tech/cosy/logger"
)

// GenerateSessionTitle generates a concise title for an LLM session based on the conversation context
func GenerateSessionTitle(messages []openai.ChatCompletionMessage) (string, error) {
	client, err := GetClient()
	if err != nil {
		return "", fmt.Errorf("failed to get LLM client: %w", err)
	}

	// Create a summarized context from the first few messages
	messageContext := extractContextForTitleGeneration(messages)
	if messageContext == "" {
		return "New Session", nil
	}

	// Prepare the system message for title generation
	systemMessage := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleSystem,
		Content: `You are a helpful assistant that generates concise, descriptive titles for chat sessions.
Based on the conversation context provided, generate a short title (2-6 words) that captures the main topic or purpose.
The title should be clear, specific, and professional.
Respond only with the title, no additional text or formatting.`,
	}

	userMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: fmt.Sprintf("Generate a title for this conversation:\n\n%s", messageContext),
	}

	req := openai.ChatCompletionRequest{
		Model:       settings.OpenAISettings.Model,
		Messages:    []openai.ChatCompletionMessage{systemMessage, userMessage},
		MaxTokens:   20, // Keep it short
		Temperature: 0.3, // Lower temperature for more consistent titles
	}

	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		logger.Error("Failed to generate session title:", err)
		return "", fmt.Errorf("failed to generate title: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "New Session", nil
	}

	title := strings.TrimSpace(resp.Choices[0].Message.Content)
	
	// Sanitize the title
	title = sanitizeTitle(title)
	
	if title == "" {
		return "New Session", nil
	}

	return title, nil
}

// extractContextForTitleGeneration extracts relevant context from messages for title generation
func extractContextForTitleGeneration(messages []openai.ChatCompletionMessage) string {
	if len(messages) == 0 {
		return ""
	}

	var contextBuilder strings.Builder
	messageCount := 0
	maxMessages := 3 // Only use the first few messages for context
	maxLength := 800  // Limit total context length

	for _, message := range messages {
		if messageCount >= maxMessages {
			break
		}

		// Skip system messages for title generation
		if message.Role == openai.ChatMessageRoleSystem {
			continue
		}

		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}

		// Add role prefix for clarity
		rolePrefix := ""
		switch message.Role {
		case openai.ChatMessageRoleUser:
			rolePrefix = "User: "
		case openai.ChatMessageRoleAssistant:
			rolePrefix = "Assistant: "
		}

		// Truncate very long messages
		if len(content) > 200 {
			content = content[:200] + "..."
		}

		newContent := fmt.Sprintf("%s%s\n", rolePrefix, content)
		
		// Check if adding this message would exceed the max length
		if contextBuilder.Len()+len(newContent) > maxLength {
			break
		}

		contextBuilder.WriteString(newContent)
		messageCount++
	}

	return contextBuilder.String()
}

// sanitizeTitle cleans up the generated title
func sanitizeTitle(title string) string {
	// Remove quotes if present
	title = strings.Trim(title, `"'`)
	
	// Remove any prefix like "Title: " if present
	if strings.HasPrefix(strings.ToLower(title), "title:") {
		title = strings.TrimSpace(title[6:])
	}
	
	// Limit length
	if len(title) > 50 {
		title = title[:47] + "..."
	}
	
	// Replace any problematic characters
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.ReplaceAll(title, "\r", " ")
	
	// Collapse multiple spaces
	for strings.Contains(title, "  ") {
		title = strings.ReplaceAll(title, "  ", " ")
	}
	
	return strings.TrimSpace(title)
}