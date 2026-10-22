package settings

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
)

// TestSaveSettingsSectionJwtSecretIsNotWritable covers GHSA-2rjr-44wx-8qpg:
// a settings write must never rotate the JWT secret. It signs every issued
// session, so a hostile write would invalidate all sessions at once and let
// the writer monopolize access after logging back in.
func TestSaveSettingsSectionJwtSecretIsNotWritable(t *testing.T) {
	confPath := useTempSettingsFile(t)
	cSettings.AppSettings.JwtSecret = "saved-secret"

	w := postSection(t, "app", `{"page_size":50,"jwt_secret":"attacker-secret"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 50, cSettings.AppSettings.PageSize)
	assert.Equal(t, "saved-secret", cSettings.AppSettings.JwtSecret)

	written, err := os.ReadFile(confPath)
	require.NoError(t, err)
	assert.NotContains(t, string(written), "attacker-secret")

	// The redacted placeholder the settings page echoes back is preserved too.
	w = postSection(t, "app", `{"page_size":10,"jwt_secret":"__NGINX_UI_REDACTED__"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "saved-secret", cSettings.AppSettings.JwtSecret)
}

// TestSaveSettingsSectionOpenAITokenStaysOnSavedEndpoint covers
// GHSA-p355-qjwh-8hgm: the stored API token must never be released to an
// endpoint, proxy or API type the settings did not already use, or a settings
// write could exfiltrate it by repointing the base URL.
func TestSaveSettingsSectionOpenAITokenStaysOnSavedEndpoint(t *testing.T) {
	useTempSettingsFile(t)
	appsettings.OpenAISettings.Token = "saved-token"
	appsettings.OpenAISettings.BaseUrl = "https://api.openai.com/v1"
	appsettings.OpenAISettings.APIType = "OPEN_AI"

	// Repointing the base URL while echoing the redacted placeholder is refused.
	w := postSection(t, "openai", `{
		"base_url":"https://attacker.example/v1",
		"token":"__NGINX_UI_REDACTED__"
	}`)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	var response struct {
		Scope string `json:"scope"`
		Code  int32  `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "settings", response.Scope)
	assert.Equal(t, int32(40005), response.Code)
	assert.Equal(t, "saved-token", appsettings.OpenAISettings.Token)
	assert.Equal(t, "https://api.openai.com/v1", appsettings.OpenAISettings.BaseUrl)

	// Repointing with a fresh token succeeds.
	w = postSection(t, "openai", `{
		"base_url":"https://attacker.example/v1",
		"token":"new-token"
	}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "new-token", appsettings.OpenAISettings.Token)
	assert.Equal(t, "https://attacker.example/v1", appsettings.OpenAISettings.BaseUrl)

	// An unchanged endpoint still restores the placeholder to the saved token.
	w = postSection(t, "openai", `{
		"base_url":"https://attacker.example/v1",
		"token":"__NGINX_UI_REDACTED__",
		"model":"gpt-5"
	}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "new-token", appsettings.OpenAISettings.Token)
	assert.Equal(t, "gpt-5", appsettings.OpenAISettings.Model)

	// A proxy change demands a fresh token as well: the proxy also sees the
	// Authorization header.
	w = postSection(t, "openai", `{
		"base_url":"https://attacker.example/v1",
		"proxy":"http://proxy.example:8080",
		"token":"__NGINX_UI_REDACTED__"
	}`)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "new-token", appsettings.OpenAISettings.Token)
	assert.Equal(t, "", appsettings.OpenAISettings.Proxy)
}
