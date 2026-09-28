package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/sashabaranov/go-openai"
)

type extraBodyKey struct{}

// WithExtraBody returns a context whose chat requests carry the given fields
// in their JSON body, the way the OpenAI SDKs send extra_body: each top-level
// field is added, replacing a field of the same name. Providers select their
// thinking modes through such fields, which go-openai has no fields for.
func WithExtraBody(ctx context.Context, extra map[string]any) context.Context {
	if len(extra) == 0 {
		return ctx
	}
	return context.WithValue(ctx, extraBodyKey{}, extra)
}

func extraBodyFrom(ctx context.Context) map[string]any {
	extra, _ := ctx.Value(extraBodyKey{}).(map[string]any)
	return extra
}

// extraBodyDoer merges the extra body of the request context into JSON
// request bodies before handing the request to next.
type extraBodyDoer struct {
	next openai.HTTPDoer
}

func (d extraBodyDoer) Do(req *http.Request) (*http.Response, error) {
	extra := extraBodyFrom(req.Context())
	if len(extra) == 0 || req.Body == nil || req.Method != http.MethodPost {
		return d.next.Do(req)
	}

	body, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, err
	}

	merged, err := mergeExtraBody(body, extra)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(merged))
	req.ContentLength = int64(len(merged))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(merged)), nil
	}

	return d.next.Do(req)
}

func mergeExtraBody(body []byte, extra map[string]any) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}

	for key, value := range extra {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		fields[key] = encoded
	}

	return json.Marshal(fields)
}
