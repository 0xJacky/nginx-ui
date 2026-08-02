package demo

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

// Canned LLM responses.
//
// The demo has no API key to spend and should not make outbound calls on a
// visitor's behalf, but an assistant panel that errors on every message shows
// the feature worse than not having it. This satisfies the same HTTPDoer the
// real client uses, so the request never leaves the process while everything
// downstream — streaming, session storage, the chat UI — runs its real code.

type llmDoer struct{}

var _ openai.HTTPDoer = (*llmDoer)(nil)

// llmReplies are matched against the user's last message, first hit wins.
// Deliberately about nginx rather than generic chit-chat: the panel sits next
// to a config editor and its answers should look like they belong there.
var llmReplies = []struct {
	match []string
	reply string
}{
	{
		[]string{"gzip", "compress"},
		"Enable compression inside the `http` block:\n\n" +
			"```nginx\ngzip on;\ngzip_comp_level 5;\ngzip_min_length 256;\ngzip_types text/plain text/css application/json application/javascript;\n```\n\n" +
			"`gzip_min_length` matters more than it looks — compressing a 40-byte response usually makes it bigger.\n\n" +
			"_This is a demo instance; replies are canned rather than generated._",
	},
	{
		[]string{"proxy", "upstream", "反向代理"},
		"A reverse proxy that preserves the client address:\n\n" +
			"```nginx\nlocation / {\n    proxy_pass http://127.0.0.1:3000;\n    proxy_set_header Host $host;\n    proxy_set_header X-Real-IP $remote_addr;\n    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n    proxy_set_header X-Forwarded-Proto $scheme;\n}\n```\n\n" +
			"Without `X-Forwarded-Proto` an application behind TLS termination will generate `http://` links.\n\n" +
			"_This is a demo instance; replies are canned rather than generated._",
	},
	{
		[]string{"websocket", "ws "},
		"WebSocket needs the upgrade headers and HTTP/1.1:\n\n" +
			"```nginx\nmap $http_upgrade $connection_upgrade {\n    default upgrade;\n    ''      close;\n}\n\nlocation /ws {\n    proxy_pass http://127.0.0.1:8080;\n    proxy_http_version 1.1;\n    proxy_set_header Upgrade $http_upgrade;\n    proxy_set_header Connection $connection_upgrade;\n}\n```\n\n" +
			"The `map` is what stops a plain request being told to upgrade.\n\n" +
			"_This is a demo instance; replies are canned rather than generated._",
	},
	{
		[]string{"ssl", "https", "certificate", "证书"},
		"A reasonable TLS baseline:\n\n" +
			"```nginx\nssl_protocols TLSv1.2 TLSv1.3;\nssl_prefer_server_ciphers off;\nssl_session_cache shared:SSL:10m;\nssl_session_timeout 1d;\n```\n\n" +
			"`ssl_prefer_server_ciphers off` is deliberate: with TLS 1.3 the client's ordering is the better one to trust.\n\n" +
			"_This is a demo instance; replies are canned rather than generated._",
	},
	{
		[]string{"cache", "缓存"},
		"Proxy caching, declared in `http` and used in a `location`:\n\n" +
			"```nginx\nproxy_cache_path /var/cache/nginx keys_zone=app:10m max_size=1g inactive=60m;\n\nlocation / {\n    proxy_cache app;\n    proxy_cache_valid 200 10m;\n    proxy_cache_use_stale error timeout updating;\n    add_header X-Cache-Status $upstream_cache_status;\n}\n```\n\n" +
			"`proxy_cache_use_stale updating` is what stops a cache miss stampeding your backend.\n\n" +
			"_This is a demo instance; replies are canned rather than generated._",
	},
}

const llmFallback = "This demo runs without an LLM provider, so answers are canned rather than generated.\n\n" +
	"Ask about **gzip**, **reverse proxying**, **WebSocket**, **SSL** or **caching** to see a real reply, " +
	"or configure a provider on your own instance to get the real assistant.\n\n" +
	"_This is a demo instance; replies are canned rather than generated._"

func replyFor(prompt string) string {
	lowered := strings.ToLower(prompt)
	for _, candidate := range llmReplies {
		for _, needle := range candidate.match {
			if strings.Contains(lowered, needle) {
				return candidate.reply
			}
		}
	}
	return llmFallback
}

// lastUserMessage pulls the prompt out of a chat completion request body.
func lastUserMessage(body io.Reader) string {
	var req openai.ChatCompletionRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return ""
	}

	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == openai.ChatMessageRoleUser {
			return req.Messages[i].Content
		}
	}
	return ""
}

func isStreaming(body []byte) bool {
	var probe struct {
		Stream bool `json:"stream"`
	}
	_ = json.Unmarshal(body, &probe)
	return probe.Stream
}

func (llmDoer) Do(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
	}

	prompt := lastUserMessage(strings.NewReader(string(body)))
	reply := replyFor(prompt)

	if !isStreaming(body) {
		return jsonResponse(req, reply)
	}
	return streamResponse(req, reply)
}

func jsonResponse(req *http.Request, reply string) (*http.Response, error) {
	payload, err := json.Marshal(openai.ChatCompletionResponse{
		ID:      "chatcmpl-demo",
		Object:  "chat.completion",
		Created: demoBuildTime.Unix(),
		Model:   "nginx-ui-demo",
		Choices: []openai.ChatCompletionChoice{{
			Index:        0,
			Message:      openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: reply},
			FinishReason: openai.FinishReasonStop,
		}},
	})
	if err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(payload))),
		Request:    req,
	}, nil
}

// streamResponse emits the server-sent-event framing go-openai's stream reader
// expects: `data: {json}` per chunk, then `data: [DONE]`.
func streamResponse(req *http.Request, reply string) (*http.Response, error) {
	reader, writer := io.Pipe()

	go func() {
		buffered := bufio.NewWriter(writer)
		defer func() {
			_ = buffered.Flush()
			_ = writer.Close()
		}()

		emit := func(content string, finish openai.FinishReason) bool {
			chunk := openai.ChatCompletionStreamResponse{
				ID:      "chatcmpl-demo",
				Object:  "chat.completion.chunk",
				Created: demoBuildTime.Unix(),
				Model:   "nginx-ui-demo",
				Choices: []openai.ChatCompletionStreamChoice{{
					Index:        0,
					Delta:        openai.ChatCompletionStreamChoiceDelta{Content: content},
					FinishReason: finish,
				}},
			}
			encoded, err := json.Marshal(chunk)
			if err != nil {
				return false
			}
			if _, err := fmt.Fprintf(buffered, "data: %s\n\n", encoded); err != nil {
				return false
			}
			return buffered.Flush() == nil
		}

		// Word by word, with a small delay, so the panel visibly streams rather
		// than snapping to a finished answer.
		for _, word := range splitKeepingSpaces(reply) {
			if !emit(word, "") {
				return
			}
			time.Sleep(12 * time.Millisecond)
		}

		if emit("", openai.FinishReasonStop) {
			_, _ = fmt.Fprint(buffered, "data: [DONE]\n\n")
		}
	}()

	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       reader,
		Request:    req,
	}, nil
}

// splitKeepingSpaces breaks text into chunks that still concatenate back into
// the original, so the streamed answer is not silently reflowed.
func splitKeepingSpaces(text string) []string {
	var out []string
	var current strings.Builder

	for _, r := range text {
		current.WriteRune(r)
		if r == ' ' || r == '\n' {
			out = append(out, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		out = append(out, current.String())
	}
	return out
}
