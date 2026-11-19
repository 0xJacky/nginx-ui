package config

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/config"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConfigBatchTaskReportsRollbackAfterReturningAccepted(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	appsettings.NginxSettings.TestConfigCmd = failingTestConfigCmd
	headers := map[string]string{"Authorization": auth.plainToken}
	router := newConfigMutationRouter()
	release := config.LockApply()
	defer func() {
		if release != nil {
			release()
		}
	}()
	response := performJSONRequest(t, router, http.MethodPost, "/config_sync_batch", gin.H{
		"async": true, "overwrite": true, "files": []gin.H{{"name": "site.conf", "content": "server {}"}},
	}, headers)
	require.Equal(t, http.StatusAccepted, response.Code)
	var task struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &task))
	require.NotEmpty(t, task.ID)
	path := "/config_sync_batch/" + task.ID
	response = performJSONRequest(t, router, http.MethodGet, path, nil, headers)
	require.Equal(t, http.StatusAccepted, response.Code)
	release()
	release = nil
	require.Eventually(t, func() bool {
		response = performJSONRequest(t, router, http.MethodGet, path, nil, headers)
		return response.Code != http.StatusAccepted
	}, 5*time.Second, 10*time.Millisecond)
	require.GreaterOrEqual(t, response.Code, 400)
	assertNginxTestFailureResponse(t, response.Body.String())
	_, err := os.Stat(filepath.Join(confDir, "site.conf"))
	require.True(t, os.IsNotExist(err))
}
