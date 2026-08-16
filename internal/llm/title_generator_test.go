package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

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

func TestSanitizeTitleKeepsMultiByteCharactersIntact(t *testing.T) {
	// 62 runes / 186 bytes: a byte-indexed cut at 47 lands inside a character.
	long := "配置 Nginx 反向代理与负载均衡的最佳实践指南大全示例文档，包含上游健康检查、缓存策略与证书自动续期的完整说明"

	got := sanitizeTitle(long)

	if !utf8.ValidString(got) {
		t.Fatalf("sanitizeTitle(%q) = %q, which is not valid UTF-8", long, got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("sanitizeTitle(%q) = %q, want a truncated value ending in an ellipsis", long, got)
	}
	if !strings.HasPrefix(long, strings.TrimSuffix(got, "...")) {
		t.Fatalf("sanitizeTitle(%q) = %q, which is not a prefix of the input", long, got)
	}
}

func TestSanitizeTitleKeepsAsciiThreshold(t *testing.T) {
	// an ASCII title is one byte per rune, so the 50-character threshold and the
	// 47-character cut must land exactly where they did before.
	for _, size := range []int{47, 48, 50, 51} {
		title := strings.Repeat("a", size)
		got := sanitizeTitle(title)

		if size <= 50 {
			if got != title {
				t.Fatalf("sanitizeTitle(%d chars) = %q, want it untouched", size, got)
			}
			continue
		}
		if got != strings.Repeat("a", 47)+"..." {
			t.Fatalf("sanitizeTitle(%d chars) = %q, want 47 characters plus an ellipsis", size, got)
		}
	}
}

func TestExtractContextForTitleGenerationKeepsMultiByteCharactersIntact(t *testing.T) {
	long := strings.Repeat("配", 300)

	got := extractContextForTitleGeneration([]openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: long},
	})

	if !utf8.ValidString(got) {
		t.Fatalf("extractContextForTitleGeneration() = %q, which is not valid UTF-8", got)
	}
}
