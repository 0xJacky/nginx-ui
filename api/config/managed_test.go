package config

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const managedSiteFileContent = "server {\n    listen 80;\n}\n"

// setupManagedSiteFile writes sites-available/ingress-web with a row managed by
// the ingress "web" and returns its path.
func setupManagedSiteFile(t *testing.T) string {
	t.Helper()

	require.NoError(t, model.UseDB().AutoMigrate(&model.Site{}))
	path, err := site.ResolveAvailablePath("ingress-web")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(managedSiteFileContent), 0o644))
	require.NoError(t, model.UseDB().Create(&model.Site{Path: path, ManagedBy: "ingress:web"}).Error)
	return path
}

func assertManagedSiteConflict(t *testing.T, code int, body []byte) {
	t.Helper()

	require.Equal(t, http.StatusConflict, code, string(body))
	var resp struct {
		Scope  string   `json:"scope"`
		Code   int32    `json:"code"`
		Params []string `json:"params"`
	}
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "site", resp.Scope)
	assert.Equal(t, int32(40901), resp.Code)
	assert.Equal(t, []string{"web"}, resp.Params)
}

func TestConfigEditorRefusesManagedSiteFileUnlessForced(t *testing.T) {
	_, auth := setupConfigSecurityTest(t)
	path := setupManagedSiteFile(t)
	router := newConfigMutationRouter()
	headers := map[string]string{"Authorization": auth.plainToken}
	edited := "server {\n    listen 8080;\n}\n"

	recorder := performJSONRequest(t, router, http.MethodPost, "/config", gin.H{
		"path": "sites-available/ingress-web", "content": edited,
	}, headers)
	assertManagedSiteConflict(t, recorder.Code, recorder.Body.Bytes())

	recorder = performJSONRequest(t, router, http.MethodPost, "/config_rename", gin.H{
		"base_path": "sites-available", "orig_name": "ingress-web", "new_name": "renamed",
	}, headers)
	assertManagedSiteConflict(t, recorder.Code, recorder.Body.Bytes())

	recorder = performJSONRequest(t, router, http.MethodPost, "/config_delete", gin.H{
		"base_path": "sites-available", "name": "ingress-web",
	}, headers)
	assertManagedSiteConflict(t, recorder.Code, recorder.Body.Bytes())

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, managedSiteFileContent, string(content))

	recorder = performJSONRequest(t, router, http.MethodPost, "/config?force=1", gin.H{
		"path": "sites-available/ingress-web", "content": edited,
	}, headers)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	content, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, edited, string(content))
}
