package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/sashabaranov/go-openai"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestGetClientBuildsMiniMaxChatCompletionURL(t *testing.T) {
	originalSettings := settings.OpenAISettings
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		settings.OpenAISettings = originalSettings
		http.DefaultTransport = originalTransport
	})

	tests := []struct {
		name    string
		config  *settings.OpenAI
		wantURL string
	}{
		{
			name: "global provider preset",
			config: &settings.OpenAI{
				Provider: settings.OpenAIProviderMiniMax,
				Token:    "test-token",
				APIType:  string(openai.APITypeOpenAI),
			},
			wantURL: settings.MiniMaxGlobalOpenAIURL + "/chat/completions",
		},
		{
			name: "china base url override",
			config: &settings.OpenAI{
				Provider: settings.OpenAIProviderMiniMax,
				BaseUrl:  settings.MiniMaxCNOpenAIURL,
				Token:    "test-token",
				APIType:  string(openai.APITypeOpenAI),
			},
			wantURL: settings.MiniMaxCNOpenAIURL + "/chat/completions",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings.OpenAISettings = test.config
			var gotURL string
			http.DefaultTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
				gotURL = request.URL.String()
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(
						`{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`,
					)),
					Request: request,
				}, nil
			})

			client, err := GetClient()
			if err != nil {
				t.Fatalf("GetClient() error = %v", err)
			}

			_, err = client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
				Model: "MiniMax-M3",
				Messages: []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleUser, Content: "Hello"},
				},
			})
			if err != nil {
				t.Fatalf("CreateChatCompletion() error = %v", err)
			}

			if gotURL != test.wantURL {
				t.Fatalf("request URL = %q, want %q", gotURL, test.wantURL)
			}
		})
	}
}
