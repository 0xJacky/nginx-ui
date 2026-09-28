package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatRequestCarriesExtraBody(t *testing.T) {
	var sent map[string]any
	withHTTPDoer(t, doerFunc(func(request *http.Request) (*http.Response, error) {
		require.NoError(t, json.NewDecoder(request.Body).Decode(&sent))
		return respond(http.StatusOK, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)(request)
	}))

	client, err := NewClient(&settings.OpenAI{
		BaseUrl: "http://localhost:11434/v1",
		APIType: string(openai.APITypeOpenAI),
	})
	require.NoError(t, err)

	ctx := WithExtraBody(context.Background(), map[string]any{
		"reasoning_effort": "high",
		"thinking":         map[string]any{"type": "enabled"},
		// An extra field replaces the field go-openai wrote
		"model": "override",
	})
	_, err = client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    "deepseek-flash",
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "hi"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "high", sent["reasoning_effort"])
	assert.Equal(t, map[string]any{"type": "enabled"}, sent["thinking"])
	assert.Equal(t, "override", sent["model"])
	assert.NotEmpty(t, sent["messages"])
}

func TestChatRequestWithoutExtraBodyIsUntouched(t *testing.T) {
	var body string
	withHTTPDoer(t, doerFunc(func(request *http.Request) (*http.Response, error) {
		raw, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		body = string(raw)
		return respond(http.StatusOK, `{"choices":[]}`)(request)
	}))

	client, err := NewClient(&settings.OpenAI{APIType: string(openai.APITypeOpenAI)})
	require.NoError(t, err)

	_, err = client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model:    "gpt-5",
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "hi"}},
	})

	require.NoError(t, err)
	assert.NotContains(t, body, "reasoning_effort")
	assert.True(t, strings.HasPrefix(body, "{"))
}
