package chatbot

import (
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestChatCompletionWithContext(t *testing.T) {
	filename := "test"
	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
		},
		{
			Role: openai.ChatMessageRoleUser,
		},
		{
			Role: openai.ChatMessageRoleAssistant,
		},
	}

	messages = ChatCompletionWithContext(filename, messages)

	assert.NotNil(t, messages[1].MultiContent)
}
