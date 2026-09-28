package llm

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStreamChunk(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		wantContent   string
		wantReasoning string
	}{
		{
			name: "returns empty strings when choices are missing",
			raw:  `{"choices":[]}`,
		},
		{
			name:        "returns delta content from first choice",
			raw:         `{"choices":[{"delta":{"content":"atlas"}}]}`,
			wantContent: "atlas",
		},
		{
			name:          "reads reasoning_content from DeepSeek and Qwen",
			raw:           `{"choices":[{"delta":{"reasoning_content":"Let me think"}}]}`,
			wantReasoning: "Let me think",
		},
		{
			name:          "reads reasoning from Ollama",
			raw:           `{"choices":[{"delta":{"content":"","reasoning":"The user asks"}}]}`,
			wantReasoning: "The user asks",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content, reasoning, err := parseStreamChunk([]byte(test.raw))
			require.NoError(t, err)
			assert.Equal(t, test.wantContent, content)
			assert.Equal(t, test.wantReasoning, reasoning)
		})
	}
}

type fakeChatStream struct {
	chunks []string
	err    error
}

func (s *fakeChatStream) RecvRaw() ([]byte, error) {
	if len(s.chunks) == 0 {
		if s.err != nil {
			return nil, s.err
		}
		return nil, io.EOF
	}
	chunk := s.chunks[0]
	s.chunks = s.chunks[1:]
	return []byte(chunk), nil
}

func collectStream(stream rawChatStream) []streamEvent {
	var events []streamEvent
	step := relayChatStream(stream, func(event streamEvent) {
		events = append(events, event)
	})
	for step(io.Discard) {
	}
	return events
}

func joinEvents(events []streamEvent, kind string) string {
	var text string
	for _, event := range events {
		if event.kind == kind {
			text += event.text
		}
	}
	return text
}

func TestRelayChatStreamSeparatesReasoningFromTheAnswer(t *testing.T) {
	events := collectStream(&fakeChatStream{chunks: []string{
		`{"choices":[{"delta":{"reasoning_content":"1 plus 1 "}}]}`,
		`{"choices":[{"delta":{"reasoning_content":"is 2."}}]}`,
		`{"choices":[{"delta":{"content":"The answer "}}]}`,
		`{"choices":[{"delta":{"content":"is 2."}}]}`,
	}})

	assert.Equal(t, "1 plus 1 is 2.", joinEvents(events, streamEventReasoning))
	assert.Equal(t, "The answer is 2.", joinEvents(events, streamEventMessage))
	require.NotEmpty(t, events)
	assert.Equal(t, streamEventReasoning, events[0].kind, "reasoning is sent before the answer")
}

func TestRelayChatStreamEndsWithTheError(t *testing.T) {
	events := collectStream(&fakeChatStream{
		chunks: []string{`{"choices":[{"delta":{"content":"partial"}}]}`},
		err:    errors.New("connection reset"),
	})

	require.Len(t, events, 2)
	assert.Equal(t, streamEvent{kind: streamEventMessage, text: "partial"}, events[0])
	assert.Equal(t, streamEvent{kind: streamEventError, text: "connection reset"}, events[1])
}
