package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}

func withHTTPDoer(t *testing.T, doer openai.HTTPDoer) {
	t.Helper()
	original := httpDoerOverride
	httpDoerOverride = doer
	t.Cleanup(func() {
		httpDoerOverride = original
	})
}

func respond(status int, body string) doerFunc {
	return func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	}
}

func TestModelKind(t *testing.T) {
	tests := map[string]string{
		"gpt-5-mini":             ModelKindChat,
		"qwen3:32b":              ModelKindChat,
		"MiniMax-M3":             ModelKindChat,
		"text-embedding-3-large": ModelKindEmbedding,
		"nomic-embed-text":       ModelKindEmbedding,
		"bge-m3":                 ModelKindEmbedding,
		"whisper-1":              ModelKindAudio,
		"tts-1-hd":               ModelKindAudio,
		"dall-e-3":               ModelKindImage,
		"gpt-image-1":            ModelKindImage,
		"omni-moderation-latest": ModelKindModeration,
	}

	for id, want := range tests {
		assert.Equal(t, want, ModelKind(id), id)
	}
}

func TestListModelsRequestsModelsEndpoint(t *testing.T) {
	var gotURL, gotAuth string
	withHTTPDoer(t, doerFunc(func(request *http.Request) (*http.Response, error) {
		gotURL = request.URL.String()
		gotAuth = request.Header.Get("Authorization")
		return respond(http.StatusOK, `{"data":[
			{"id":"whisper-1","owned_by":"openai"},
			{"id":"gpt-5","owned_by":"openai"},
			{"id":"gpt-5","owned_by":"openai"},
			{"id":" ","owned_by":"openai"}
		]}`)(request)
	}))

	models, err := ListModels(context.Background(), &settings.OpenAI{
		Provider: settings.OpenAIProviderCustom,
		BaseUrl:  "http://localhost:11434/v1/",
		Token:    "test-token",
		APIType:  string(openai.APITypeOpenAI),
	})

	require.NoError(t, err)
	assert.Equal(t, "http://localhost:11434/v1/models", gotURL)
	assert.Equal(t, "Bearer test-token", gotAuth)
	assert.Equal(t, []Model{
		{ID: "gpt-5", OwnedBy: "openai", Kind: ModelKindChat},
		{ID: "whisper-1", OwnedBy: "openai", Kind: ModelKindAudio},
	}, models)
}

func TestListModelsClassifiesFailures(t *testing.T) {
	tests := []struct {
		name string
		doer doerFunc
		want error
	}{
		{
			name: "unauthorized",
			doer: respond(http.StatusUnauthorized, `{"error":{"message":"Incorrect API key provided"}}`),
			want: ErrModelListUnauthorized,
		},
		{
			name: "forbidden without error body",
			doer: respond(http.StatusForbidden, `forbidden`),
			want: ErrModelListUnauthorized,
		},
		{
			name: "no models endpoint",
			doer: respond(http.StatusNotFound, `404 page not found`),
			want: ErrModelListUnsupported,
		},
		{
			name: "web page instead of a model list",
			doer: respond(http.StatusOK, `<!doctype html><html></html>`),
			want: ErrModelListUnsupported,
		},
		{
			name: "upstream failure",
			doer: respond(http.StatusBadGateway, `{"error":{"message":"upstream is down"}}`),
			want: ErrModelListUpstream,
		},
		{
			name: "timeout",
			doer: func(*http.Request) (*http.Response, error) {
				return nil, context.DeadlineExceeded
			},
			want: ErrModelListTimeout,
		},
		{
			name: "connection refused",
			doer: func(*http.Request) (*http.Response, error) {
				return nil, errors.New("dial tcp 127.0.0.1:11434: connect: connection refused")
			},
			want: ErrModelListUnreachable,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withHTTPDoer(t, test.doer)

			_, err := ListModels(context.Background(), &settings.OpenAI{
				BaseUrl: "http://localhost:11434/v1",
				APIType: string(openai.APITypeOpenAI),
			})

			var cErr, wantErr *cosy.Error
			require.ErrorAs(t, err, &cErr)
			require.ErrorAs(t, test.want, &wantErr)
			assert.Equal(t, wantErr.Scope, cErr.Scope)
			assert.Equal(t, wantErr.Code, cErr.Code)
		})
	}
}

func TestListModelsUpstreamErrorKeepsStatus(t *testing.T) {
	withHTTPDoer(t, respond(http.StatusBadGateway, `{"error":{"message":"upstream is down"}}`))

	_, err := ListModels(context.Background(), &settings.OpenAI{
		BaseUrl: "http://localhost:11434/v1",
		APIType: string(openai.APITypeOpenAI),
	})

	var cErr *cosy.Error
	require.ErrorAs(t, err, &cErr)
	assert.Equal(t, []string{"502 Bad Gateway: upstream is down"}, cErr.Params)
}

func TestListModelsRejectsAzure(t *testing.T) {
	withHTTPDoer(t, doerFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("an Azure model list must not reach the network")
		return nil, nil
	}))

	_, err := ListModels(context.Background(), &settings.OpenAI{
		BaseUrl: "https://example.openai.azure.com",
		APIType: string(openai.APITypeAzure),
	})

	assert.ErrorIs(t, err, ErrModelListUnsupported)
}
