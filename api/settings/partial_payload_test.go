package settings

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postSettings(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()

	r := gin.New()
	r.POST("/api/settings", SaveSettings)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// TestSaveSettingsPartialBodyKeepsOmittedSections covers the follow-up found
// on issue #2054: posting only the section being changed reset every other
// section, because the payload started from zero values and ProtectedFill
// copied all of them over the stored settings.
func TestSaveSettingsPartialBodyKeepsOmittedSections(t *testing.T) {
	useTempSettingsFile(t)
	originalHTTP := *appsettings.HTTPSettings
	t.Cleanup(func() { *appsettings.HTTPSettings = originalHTTP })

	appsettings.OpenAISettings.Token = "saved-token"
	appsettings.OpenAISettings.BaseUrl = "https://api.openai.com/v1"
	appsettings.OpenAISettings.APIType = "OPEN_AI"
	appsettings.OpenAISettings.Model = "saved-model"
	appsettings.HTTPSettings.HTTPProxy = "http://saved-proxy.example:3128"
	appsettings.HTTPSettings.WebSocketTrustedOrigins = []string{"https://saved.example"}

	w := postSettings(t, `{"http":{"github_proxy":"https://mirror.example"}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Equal(t, "https://mirror.example", appsettings.HTTPSettings.GithubProxy)
	assert.Equal(t, "http://saved-proxy.example:3128", appsettings.HTTPSettings.HTTPProxy,
		"an omitted field of a posted section must be kept")
	assert.Equal(t, []string{"https://saved.example"}, appsettings.HTTPSettings.WebSocketTrustedOrigins)
	assert.Equal(t, "saved-model", appsettings.OpenAISettings.Model, "an omitted section must be kept")
	assert.Equal(t, "saved-token", appsettings.OpenAISettings.Token)
	assert.Equal(t, "https://api.openai.com/v1", appsettings.OpenAISettings.BaseUrl)
}

func TestSaveSettingsSectionPartialBodyKeepsOmittedFields(t *testing.T) {
	useTempSettingsFile(t)
	appsettings.OpenAISettings.Token = "saved-token"
	appsettings.OpenAISettings.BaseUrl = "https://api.openai.com/v1"
	appsettings.OpenAISettings.APIType = "OPEN_AI"
	appsettings.OpenAISettings.Model = "saved-model"

	w := postSection(t, "openai", `{"model":"new-model"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Equal(t, "new-model", appsettings.OpenAISettings.Model)
	assert.Equal(t, "https://api.openai.com/v1", appsettings.OpenAISettings.BaseUrl)
	assert.Equal(t, "OPEN_AI", appsettings.OpenAISettings.APIType)
	assert.Equal(t, "saved-token", appsettings.OpenAISettings.Token,
		"an omitted token on the saved endpoint must be kept")
}

// An omitted token must not be released to a new endpoint either: it is
// treated exactly like the redacted placeholder (GHSA-p355-qjwh-8hgm).
func TestSaveSettingsSectionOmittedTokenStaysOnSavedEndpoint(t *testing.T) {
	useTempSettingsFile(t)
	appsettings.OpenAISettings.Token = "saved-token"
	appsettings.OpenAISettings.BaseUrl = "https://api.openai.com/v1"
	appsettings.OpenAISettings.APIType = "OPEN_AI"

	w := postSection(t, "openai", `{"base_url":"https://attacker.example/v1"}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	assert.Equal(t, "saved-token", appsettings.OpenAISettings.Token)
	assert.Equal(t, "https://api.openai.com/v1", appsettings.OpenAISettings.BaseUrl)
}

func TestPrefillSettingsPayloadDoesNotShareSlices(t *testing.T) {
	current := &appsettings.HTTP{WebSocketTrustedOrigins: make([]string, 1, 4)}
	current.WebSocketTrustedOrigins[0] = "https://saved.example"

	payload := prefillSettingsPayload(current)
	payload.WebSocketTrustedOrigins[0] = "https://changed.example"
	_ = append(payload.WebSocketTrustedOrigins[:0], "https://appended.example")

	assert.Equal(t, []string{"https://saved.example"}, current.WebSocketTrustedOrigins,
		"binding into the payload must not edit the live settings")
}

func TestPrefillSettingsPayloadRedactsSensitiveStrings(t *testing.T) {
	payload := prefillSettingsPayload(&appsettings.OpenAI{Token: "saved-token", Model: "saved-model"})

	assert.Equal(t, redactedSensitiveValue, payload.Token)
	assert.Equal(t, "saved-model", payload.Model)
}
