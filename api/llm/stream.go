package llm

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

// Kinds of events relayed to the chat panel.
const (
	streamEventMessage   = "message"
	streamEventReasoning = "reasoning"
	streamEventError     = "error"
)

// streamFlushInterval batches the provider's small deltas into fewer events.
const streamFlushInterval = 500 * time.Millisecond

type streamEvent struct {
	kind string
	text string
}

// rawChatStream is the part of the go-openai chat stream the relay reads.
type rawChatStream interface {
	RecvRaw() ([]byte, error)
}

// streamChunk is one chat completion chunk. Providers send the reasoning of a
// thinking model in different fields: reasoning_content (DeepSeek, Qwen,
// Volcengine Ark) or reasoning (Ollama, OpenRouter, vLLM).
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			Reasoning        string `json:"reasoning"`
		} `json:"delta"`
	} `json:"choices"`
}

func parseStreamChunk(raw []byte) (content, reasoning string, err error) {
	var chunk streamChunk
	if err = json.Unmarshal(raw, &chunk); err != nil {
		return "", "", err
	}
	if len(chunk.Choices) == 0 {
		return "", "", nil
	}

	delta := chunk.Choices[0].Delta
	reasoning = delta.ReasoningContent
	if reasoning == "" {
		reasoning = delta.Reasoning
	}
	return delta.Content, reasoning, nil
}

// relayChatStream reads the provider stream and returns a step function for
// gin's Context.Stream that emits the reasoning and answer in batches, each
// kind in its own event, followed by an error event if the stream fails.
func relayChatStream(stream rawChatStream, emit func(streamEvent)) func(io.Writer) bool {
	events := make(chan streamEvent)

	go func() {
		defer close(events)
		deltas := make(chan streamEvent)

		go func() {
			defer close(deltas)
			for {
				raw, err := stream.RecvRaw()
				if errors.Is(err, io.EOF) {
					return
				}
				if err != nil {
					logger.Errorf("Stream error: %v", err)
					deltas <- streamEvent{kind: streamEventError, text: err.Error()}
					return
				}

				content, reasoning, err := parseStreamChunk(raw)
				if err != nil {
					logger.Errorf("Stream chunk error: %v", err)
					continue
				}
				if reasoning != "" {
					deltas <- streamEvent{kind: streamEventReasoning, text: reasoning}
				}
				if content != "" {
					deltas <- streamEvent{kind: streamEventMessage, text: content}
				}
			}
		}()

		ticker := time.NewTicker(streamFlushInterval)
		defer ticker.Stop()

		var reasoning, content strings.Builder
		flush := func() {
			if reasoning.Len() > 0 {
				events <- streamEvent{kind: streamEventReasoning, text: reasoning.String()}
				reasoning.Reset()
			}
			if content.Len() > 0 {
				events <- streamEvent{kind: streamEventMessage, text: content.String()}
				content.Reset()
			}
		}

		for {
			select {
			case delta, ok := <-deltas:
				if !ok {
					flush()
					return
				}
				switch delta.kind {
				case streamEventError:
					flush()
					events <- delta
					return
				case streamEventReasoning:
					reasoning.WriteString(delta.text)
				default:
					content.WriteString(delta.text)
				}
			case <-ticker.C:
				flush()
			}
		}
	}()

	return func(io.Writer) bool {
		event, ok := <-events
		if !ok {
			return false
		}
		emit(event)
		return event.kind != streamEventError
	}
}
