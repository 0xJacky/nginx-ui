package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestCtlClientBuildsSafeAPIURL(t *testing.T) {
	baseURL, err := url.Parse("https://example.test/nginx-ui")
	require.NoError(t, err)
	client := &ctlClient{baseURL: baseURL}

	requestURL, err := client.apiURL("users?page=2")
	require.NoError(t, err)
	assert.Equal(t, "https://example.test/nginx-ui/api/users?page=2", requestURL.String())

	requestURL, err = client.apiURL("/api/nginx/status")
	require.NoError(t, err)
	assert.Equal(t, "https://example.test/api/nginx/status", requestURL.String())

	_, err = client.apiURL("https://attacker.test/api/users")
	require.Error(t, err)
}

func TestCtlClientDoesNotFollowRedirects(t *testing.T) {
	redirected := false
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Host == "attacker.test" {
				redirected = true
			}
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"https://attacker.test/capture"}},
				Body:       io.NopCloser(strings.NewReader("redirect")),
				Request:    request,
			}, nil
		}),
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	baseURL, err := url.Parse("https://nginx-ui.test")
	require.NoError(t, err)
	ctl := &ctlClient{baseURL: baseURL, token: "nui_pat_test", client: client}

	_, err = ctl.do(context.Background(), http.MethodGet, "users", nil)
	var httpError *ctlHTTPError
	require.ErrorAs(t, err, &httpError)
	assert.Equal(t, http.StatusFound, httpError.StatusCode)
	assert.False(t, redirected)
}

func TestCtlClientSendsBearerTokenNodeAndJSON(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "/api/users", request.URL.Path)
		assert.Equal(t, "Bearer nui_pat_test", request.Header.Get("Authorization"))
		assert.Equal(t, "42", request.Header.Get("X-Node-ID"))
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"name":"automation"}`, string(body))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":9}`)),
		}, nil
	})

	baseURL, err := url.Parse("https://nginx-ui.test")
	require.NoError(t, err)
	client := &ctlClient{
		baseURL: baseURL,
		token:   "nui_pat_test",
		nodeID:  "42",
		client:  &http.Client{Transport: transport, Timeout: time.Second},
		stdout:  io.Discard,
	}
	body, err := client.do(context.Background(), http.MethodPost, "users", map[string]string{"name": "automation"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":9}`, string(body))
}

func TestCtlClientReturnsBoundedHTTPError(t *testing.T) {
	baseURL, err := url.Parse("https://nginx-ui.test")
	require.NoError(t, err)
	client := &ctlClient{baseURL: baseURL, token: "bad", client: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"message":"Authorization failed"}`)),
		}, nil
	})}}

	_, err = client.do(context.Background(), http.MethodGet, "users", nil)
	var httpError *ctlHTTPError
	require.ErrorAs(t, err, &httpError)
	assert.Equal(t, http.StatusForbidden, httpError.StatusCode)
}

func TestWriteCLIResponseFormatsJSON(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, writeCLIResponse(&output, []byte(`{"ok":true}`)))
	var decoded map[string]bool
	require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
	assert.True(t, decoded["ok"])
}

func TestRedactJSONFieldsRecursivelyRemovesCertificateMaterial(t *testing.T) {
	redacted, err := redactJSONFields([]byte(`{
		"data":[{"id":1,"ssl_certificate":"cert","ssl_certificate_key":"key"}],
		"ssl_certificate_key":"top-level-key"
	}`), "ssl_certificate", "ssl_certificate_key")
	require.NoError(t, err)
	assert.JSONEq(t, `{"data":[{"id":1}]}`, string(redacted))
	assert.NotContains(t, string(redacted), "cert")
	assert.NotContains(t, string(redacted), "key")
}

func TestCtlPasswordLimitCountsUnicodeCharacters(t *testing.T) {
	password := strings.Repeat("密", 20)
	value, err := normalizeCtlPassword([]byte(password + "\n"))
	require.NoError(t, err)
	assert.Equal(t, password, value)

	_, err = normalizeCtlPassword([]byte(strings.Repeat("密", 21)))
	require.ErrorContains(t, err, "20 characters")
}
