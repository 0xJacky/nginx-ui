package settings

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
)

// useTempSettingsFile points settings writes at a scratch app.ini so a save
// can run end to end without touching the developer's configuration.
func useTempSettingsFile(t *testing.T) string {
	t.Helper()

	confPath := filepath.Join(t.TempDir(), "app.ini")
	require.NoError(t, os.WriteFile(confPath, []byte("[app]\n"), 0o600))

	originalPath := cSettings.ConfPath
	originalConf := cSettings.Conf
	originalOpenAI := *appsettings.OpenAISettings
	originalLogrotate := *appsettings.LogrotateSettings
	originalServer := *cSettings.ServerSettings
	t.Cleanup(func() {
		cSettings.ConfPath = originalPath
		cSettings.Conf = originalConf
		*appsettings.OpenAISettings = originalOpenAI
		*appsettings.LogrotateSettings = originalLogrotate
		*cSettings.ServerSettings = originalServer
	})

	cSettings.ConfPath = confPath
	require.NoError(t, cSettings.Reload())
	return confPath
}

func postSection(t *testing.T, section string, body string) *httptest.ResponseRecorder {
	t.Helper()

	r := gin.New()
	r.POST("/api/settings/:section", SaveSettingsSection)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings/"+section, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestSaveSettingsSectionSavesOnlyThatSection(t *testing.T) {
	confPath := useTempSettingsFile(t)
	appsettings.OpenAISettings.Token = "saved-token"
	appsettings.LogrotateSettings.Interval = 1440

	w := postSection(t, "openai", `{
		"provider":"custom",
		"base_url":"http://localhost:11434/v1",
		"token":"__NGINX_UI_REDACTED__",
		"model":"qwen3:32b",
		"api_type":"OPEN_AI"
	}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "qwen3:32b", response["model"])
	assert.Equal(t, redactedSensitiveValue, response["token"])
	assert.NotContains(t, response, "openai", "the response is the section itself")

	assert.Equal(t, "saved-token", appsettings.OpenAISettings.Token)
	assert.Equal(t, "qwen3:32b", appsettings.OpenAISettings.Model)
	assert.Equal(t, 1440, appsettings.LogrotateSettings.Interval)

	written, err := os.ReadFile(confPath)
	require.NoError(t, err)
	assert.Contains(t, string(written), "qwen3:32b")
}

func TestSaveSettingsSectionReportsFieldErrorsWithinTheSection(t *testing.T) {
	useTempSettingsFile(t)
	appsettings.OpenAISettings.Model = "unchanged"

	w := postSection(t, "openai", `{"base_url":"not a url","model":"gpt-5"}`)

	require.Equal(t, http.StatusNotAcceptable, w.Code)
	var response struct {
		Errors map[string]any `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, map[string]any{"base_url": "url"}, response.Errors)
	assert.Equal(t, "unchanged", appsettings.OpenAISettings.Model)
}

func TestSaveSettingsSectionRejectsInvalidServerCombination(t *testing.T) {
	useTempSettingsFile(t)
	cSettings.ServerSettings.EnableH2 = false

	w := postSection(t, "server", `{"host":"0.0.0.0","port":9000,"enable_https":false,"enable_h2":true}`)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var response struct {
		Scope string `json:"scope"`
		Code  int32  `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "settings", response.Scope)
	assert.Equal(t, int32(40001), response.Code)
	assert.False(t, cSettings.ServerSettings.EnableH2)
}

func TestSaveSettingsSectionRejectsUnknownSection(t *testing.T) {
	for _, section := range []string{"listener", "database", "webauthn", "nope"} {
		t.Run(section, func(t *testing.T) {
			w := postSection(t, section, `{}`)

			require.Equal(t, http.StatusNotFound, w.Code)
			assert.Contains(t, w.Body.String(), `"code":40004`)
		})
	}
}

func TestSettingsSectionsCoverTheCombinedPayload(t *testing.T) {
	// Every group the combined endpoint writes must also be writable on its
	// own, or the preference page could not save it.
	payloadType := reflectTypeOf[saveSettingsPayload]()
	for i := 0; i < payloadType.NumField(); i++ {
		name := jsonFieldName(payloadType.Field(i))
		assert.Contains(t, sectionHandlerMap, name)
		assert.Contains(t, settingsResponseBuilders, name)
	}
	assert.Len(t, sectionHandlerMap, payloadType.NumField())
}

func reflectTypeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}
