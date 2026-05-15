package llm

import (
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
)

func TestGetStreamDeltaContent(t *testing.T) {
	t.Run("returns empty string when choices are missing", func(t *testing.T) {
		response := openai.ChatCompletionStreamResponse{}

		assert.Empty(t, getStreamDeltaContent(response))
	})

	t.Run("returns delta content from first choice", func(t *testing.T) {
		response := openai.ChatCompletionStreamResponse{
			Choices: []openai.ChatCompletionStreamChoice{
				{
					Delta: openai.ChatCompletionStreamChoiceDelta{
						Content: "atlas",
					},
				},
			},
		}

		assert.Equal(t, "atlas", getStreamDeltaContent(response))
	})
}
