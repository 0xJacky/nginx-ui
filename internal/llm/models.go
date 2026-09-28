package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/sashabaranov/go-openai"
	"github.com/uozi-tech/cosy"
)

// Model kinds let the settings page list chat models first and fold the rest
// away. The kind is a guess from the model id, since the OpenAI-compatible
// /models endpoint does not say what a model is for.
const (
	ModelKindChat       = "chat"
	ModelKindEmbedding  = "embedding"
	ModelKindAudio      = "audio"
	ModelKindImage      = "image"
	ModelKindModeration = "moderation"
)

const modelListTimeout = 10 * time.Second

// Model is one entry of a provider's model list.
type Model struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by"`
	Kind    string `json:"kind"`
}

// modelKindKeywords is checked in order, the first kind with a matching
// keyword wins.
var modelKindKeywords = []struct {
	kind     string
	keywords []string
}{
	{ModelKindModeration, []string{"moderation"}},
	{ModelKindEmbedding, []string{"embed", "rerank", "bge-", "bge:"}},
	{ModelKindAudio, []string{"whisper", "tts", "transcribe", "speech"}},
	{ModelKindImage, []string{"dall-e", "image", "stable-diffusion", "sdxl", "flux"}},
}

// ModelKind guesses what a model is used for from its id.
func ModelKind(id string) string {
	lowered := strings.ToLower(id)
	for _, candidate := range modelKindKeywords {
		for _, keyword := range candidate.keywords {
			if strings.Contains(lowered, keyword) {
				return candidate.kind
			}
		}
	}
	return ModelKindChat
}

// ListModels asks the provider described by options for its models. Failures
// are returned as llm scope errors that tell the user which part of the
// connection to fix.
func ListModels(ctx context.Context, options *settings.OpenAI) ([]Model, error) {
	// Azure lists deployments through its management API rather than /models.
	if openai.APIType(options.APIType) == openai.APITypeAzure {
		return nil, ErrModelListUnsupported
	}

	client, err := NewClient(options)
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrModelListUnreachable, err.Error())
	}

	ctx, cancel := context.WithTimeout(ctx, modelListTimeout)
	defer cancel()

	list, err := client.ListModels(ctx)
	if err != nil {
		return nil, classifyModelListError(err)
	}

	models := make([]Model, 0, len(list.Models))
	seen := make(map[string]struct{}, len(list.Models))
	for _, model := range list.Models {
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, Model{
			ID:      id,
			OwnedBy: model.OwnedBy,
			Kind:    ModelKind(id),
		})
	}

	slices.SortFunc(models, func(a, b Model) int {
		return strings.Compare(a.ID, b.ID)
	})

	return models, nil
}

func classifyModelListError(err error) error {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return modelListStatusError(apiErr.HTTPStatusCode, apiErr.Message)
	}

	var requestErr *openai.RequestError
	if errors.As(err, &requestErr) {
		return modelListStatusError(requestErr.HTTPStatusCode, requestErr.HTTPStatus)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return ErrModelListTimeout
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrModelListTimeout
	}

	// A 200 answer that is not a model list, such as a web page served at a
	// base URL missing its /v1 suffix.
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &syntaxErr) || errors.As(err, &typeErr) {
		return ErrModelListUnsupported
	}

	return cosy.WrapErrorWithParams(ErrModelListUnreachable, err.Error())
}

func modelListStatusError(status int, detail string) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrModelListUnauthorized
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		return ErrModelListUnsupported
	}

	message := fmt.Sprintf("%d %s", status, http.StatusText(status))
	if detail = strings.TrimSpace(detail); detail != "" && !strings.Contains(detail, http.StatusText(status)) {
		message += ": " + detail
	}
	return cosy.WrapErrorWithParams(ErrModelListUpstream, message)
}
