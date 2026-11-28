package middleware

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func serviceTokenShapeResponse(contentType, body string) *http.Response {
	response := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
	if contentType != "" {
		response.Header.Set("Content-Type", contentType)
	}
	return response
}

// The service token redaction covers every answer shape of a redacted route
// and relays an answer unchanged only when it cannot hold a JSON field.
func TestRedactServiceTokenProxyResponseShapes(t *testing.T) {
	const route = "/api/auto_backup"
	const jsonType = "application/json; charset=utf-8"
	const secret = "SHAPE-SECRET"

	redacted := []struct {
		name, contentType, body string
	}{
		{"list", jsonType, `{"data":[{"name":"a","s3_secret_access_key":"` + secret + `"}],"pagination":{"total":1}}`},
		{"single record", jsonType, `{"id":1,"name":"a","storage_config":{"k":"` + secret + `"}}`},
		{"record under data", jsonType, `{"data":{"name":"a","s3_access_key_id":"` + secret + `"}}`},
		{"bare array", jsonType, `[{"name":"a","s3_secret_access_key":"` + secret + `"}]`},
		{"JSON declared as text", "text/plain", `{"name":"a","s3_secret_access_key":"` + secret + `"}`},
		{"JSON without a content type", "", `{"name":"a","s3_secret_access_key":"` + secret + `"}`},
	}
	for _, tc := range redacted {
		t.Run(tc.name, func(t *testing.T) {
			response := serviceTokenShapeResponse(tc.contentType, tc.body)
			require.NoError(t, redactServiceTokenProxyResponse(response, route))
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), `"name":"a"`)
			require.NotContains(t, string(body), secret)
			require.EqualValues(t, len(body), response.ContentLength)
		})
	}

	unchanged := []struct {
		name, contentType, body string
	}{
		{"no content", jsonType, ""},
		{"whitespace", jsonType, " \n"},
		{"plain text", "text/plain; charset=utf-8", "ok"},
	}
	for _, tc := range unchanged {
		t.Run(tc.name, func(t *testing.T) {
			response := serviceTokenShapeResponse(tc.contentType, tc.body)
			require.NoError(t, redactServiceTokenProxyResponse(response, route))
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.Equal(t, tc.body, string(body))
		})
	}

	refused := []struct {
		name, contentType, body string
	}{
		{"malformed JSON", jsonType, `{"s3_secret_access_key":"` + secret},
		{"malformed without a content type", "", `{"s3_secret_access_key":"` + secret},
		{"list item that is not a record", jsonType, `{"data":["` + secret + `"]}`},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			response := serviceTokenShapeResponse(tc.contentType, tc.body)
			require.Error(t, redactServiceTokenProxyResponse(response, route))
		})
	}

	t.Run("encoded body declared as text", func(t *testing.T) {
		response := serviceTokenShapeResponse("text/plain", "\x1f\x8b compressed")
		response.Header.Set("Content-Encoding", "gzip")
		require.Error(t, redactServiceTokenProxyResponse(response, route))
	})

	t.Run("no body", func(t *testing.T) {
		response := &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: http.NoBody}
		require.NoError(t, redactServiceTokenProxyResponse(response, route))
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Empty(t, body)
	})

	t.Run("route without redaction", func(t *testing.T) {
		response := serviceTokenShapeResponse(jsonType, `{"secret":"`+secret+`"}`)
		require.NoError(t, redactServiceTokenProxyResponse(response, "/api/unlisted"))
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), secret)
	})
}
