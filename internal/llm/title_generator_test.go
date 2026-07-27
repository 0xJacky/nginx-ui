package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/sashabaranov/go-openai"
)

func TestGenerateSessionTitleUsesMaxCompletionTokens(t *testing.T) {
	originalSettings := settings.OpenAISettings
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		settings.OpenAISettings = originalSettings
		http.DefaultTransport = originalTransport
	})

	settings.OpenAISettings = &settings.OpenAI{
		Token:   "test-token",
		Model:   "gpt-5-mini",
		APIType: string(openai.APITypeOpenAI),
	}

	var requestBody map[string]any
	http.DefaultTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"choices":[{"message":{"role":"assistant","content":"NGINX Configuration"},"finish_reason":"stop"}]}`,
			)),
			Request: request,
		}, nil
	})

	title, err := GenerateSessionTitle([]openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "Help me configure NGINX."},
	})
	if err != nil {
		t.Fatalf("GenerateSessionTitle() error = %v", err)
	}
	if title != "NGINX Configuration" {
		t.Fatalf("GenerateSessionTitle() = %q, want %q", title, "NGINX Configuration")
	}
	if got := requestBody["max_completion_tokens"]; got != float64(20) {
		t.Fatalf("max_completion_tokens = %#v, want 20", got)
	}
	if _, exists := requestBody["max_tokens"]; exists {
		t.Fatal("request contains deprecated max_tokens")
	}
	if _, exists := requestBody["temperature"]; exists {
		t.Fatal("request contains a temperature unsupported by reasoning models")
	}
}
