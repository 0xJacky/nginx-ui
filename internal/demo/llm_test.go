package demo

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chatRequest(t *testing.T, prompt string, stream bool) *http.Request {
	t.Helper()

	body, err := json.Marshal(openai.ChatCompletionRequest{
		Model:  "gpt-4",
		Stream: stream,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "you are an nginx assistant"},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", strings.NewReader(string(body)))
	require.NoError(t, err)
	return req
}

func TestLLMDoerNeverLeavesTheProcess(t *testing.T) {
	// The point of the override: an outbound request must not happen. If Do
	// ever returned an error instead of a response, the panel would show a
	// failure and the demo would look broken.
	resp, err := llmDoer{}.Do(chatRequest(t, "how do I enable gzip?", false))

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestLLMDoerAnswersNonStreaming(t *testing.T) {
	resp, err := llmDoer{}.Do(chatRequest(t, "how do I enable gzip?", false))
	require.NoError(t, err)
	defer resp.Body.Close()

	var decoded openai.ChatCompletionResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))
	require.Len(t, decoded.Choices, 1)
	assert.Contains(t, decoded.Choices[0].Message.Content, "gzip on;")
	assert.Equal(t, openai.FinishReasonStop, decoded.Choices[0].FinishReason)
}

func TestLLMDoerStreamFramingMatchesWhatTheReaderExpects(t *testing.T) {
	// go-openai's stream reader wants `data: {json}` per chunk and a final
	// `data: [DONE]`. Getting the separator wrong makes the panel hang with no
	// error anywhere, so assert the wire format directly.
	resp, err := llmDoer{}.Do(chatRequest(t, "set up a reverse proxy", true))
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	body := string(raw)

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.True(t, strings.HasPrefix(body, "data: "), "stream must start with a data frame")
	assert.True(t, strings.HasSuffix(body, "data: [DONE]\n\n"), "stream must terminate with [DONE]")

	var assembled strings.Builder
	var sawStop bool
	for _, line := range strings.Split(body, "\n\n") {
		payload, ok := strings.CutPrefix(line, "data: ")
		if !ok || payload == "[DONE]" {
			continue
		}

		var chunk openai.ChatCompletionStreamResponse
		require.NoError(t, json.Unmarshal([]byte(payload), &chunk), payload)
		require.Len(t, chunk.Choices, 1)
		assembled.WriteString(chunk.Choices[0].Delta.Content)
		if chunk.Choices[0].FinishReason == openai.FinishReasonStop {
			sawStop = true
		}
	}

	assert.True(t, sawStop, "a stream with no stop reason never completes for the client")
	// Chunking must be lossless, or the answer arrives silently reflowed.
	assert.Equal(t, replyFor("set up a reverse proxy"), assembled.String())
}

func TestLLMRepliesMatchOnTopic(t *testing.T) {
	cases := map[string]string{
		"how do I turn on gzip":        "gzip on;",
		"help me with a reverse proxy": "proxy_pass",
		"my websocket keeps failing":   "Upgrade",
		"what ssl protocols":           "ssl_protocols",
		"how do I cache responses":     "proxy_cache",
	}

	for prompt, expected := range cases {
		assert.Contains(t, replyFor(prompt), expected, prompt)
	}
}

func TestLLMFallsBackWhenOffTopic(t *testing.T) {
	reply := replyFor("what is the capital of France")

	assert.Equal(t, llmFallback, reply)
	assert.Contains(t, reply, "canned")
}

func TestLLMAlwaysDisclosesThatItIsCanned(t *testing.T) {
	// A demo that silently pretends to be a real assistant is misleading.
	for _, prompt := range []string{"gzip", "proxy", "websocket", "ssl", "cache", "something else"} {
		assert.Contains(t, strings.ToLower(replyFor(prompt)), "canned", prompt)
	}
}

func TestSplitKeepingSpacesIsLossless(t *testing.T) {
	for _, text := range []string{
		"hello world",
		"line one\nline two\n",
		"```nginx\ngzip on;\n```",
		"",
		"   ",
	} {
		assert.Equal(t, text, strings.Join(splitKeepingSpaces(text), ""), text)
	}
}

func TestLastUserMessagePicksTheMostRecent(t *testing.T) {
	body, err := json.Marshal(openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "first"},
			{Role: openai.ChatMessageRoleAssistant, Content: "reply"},
			{Role: openai.ChatMessageRoleUser, Content: "second"},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "second", lastUserMessage(strings.NewReader(string(body))))
	assert.Empty(t, lastUserMessage(strings.NewReader("not json")))
}
