package chatbot

import (
	"github.com/sashabaranov/go-openai"
)

func ChatCompletionWithContext(filename string, messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == openai.ChatMessageRoleUser {
			// openai.ChatCompletionMessage: can't use both Content and MultiContent properties simultaneously
			multiContent := getConfigIncludeContext(filename)
			multiContent = append(multiContent, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: messages[i].Content,
			})
			messages[i].Content = ""
			messages[i].MultiContent = multiContent
		}
	}
	return messages
}
