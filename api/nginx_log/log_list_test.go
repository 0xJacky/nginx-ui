package nginx_log

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLogListGroupsFiltersAndSortsWithoutIndexData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	nginx_log.AddLogPath("/var/log/nginx/api-test-a.access.log", "access", "api-test-a.access.log", "/etc/nginx/api-test.conf")
	nginx_log.AddLogPath("/var/log/nginx/api-test-a.access.log.1", "access", "api-test-a.access.log.1", "/etc/nginx/api-test.conf")
	nginx_log.AddLogPath("/var/log/nginx/api-test-b.error.log", "error", "api-test-b.error.log", "/etc/nginx/api-test.conf")
	t.Cleanup(func() { nginx_log.RemoveLogPathsFromConfig("/etc/nginx/api-test.conf") })

	list := func(query string) map[string]any {
		t.Helper()
		router := gin.New()
		router.GET("/nginx_logs", GetLogList)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/nginx_logs?"+query, nil))
		require.Equal(t, http.StatusOK, recorder.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		return body
	}

	body := list("name=api-test&sort_by=name")
	assert.NotContains(t, body, "summary")
	data, ok := body["data"].([]any)
	require.True(t, ok)
	require.Len(t, data, 2, "the rotated file folds into its main log")
	first := data[0].(map[string]any)
	assert.Equal(t, "/var/log/nginx/api-test-a.access.log", first["path"])
	assert.Equal(t, "access", first["type"])
	assert.Equal(t, "/etc/nginx/api-test.conf", first["config_file"])
	assert.NotContains(t, first, "index_status")

	errors := list("name=api-test&type=error")["data"].([]any)
	require.Len(t, errors, 1)
	assert.Equal(t, "/var/log/nginx/api-test-b.error.log", errors[0].(map[string]any)["path"])
}

func TestGetLegacyIndexingStatusReportsTheReadOnlyFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)

	previous := settings.NginxLogSettings.IndexingEnabled
	t.Cleanup(func() { settings.NginxLogSettings.IndexingEnabled = previous })

	for _, enabled := range []bool{true, false} {
		settings.NginxLogSettings.IndexingEnabled = enabled

		router := gin.New()
		router.GET("/nginx_log/legacy_indexing", GetLegacyIndexingStatus)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/nginx_log/legacy_indexing", nil))
		require.Equal(t, http.StatusOK, recorder.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		assert.Equal(t, enabled, body["enabled"])
	}
}
