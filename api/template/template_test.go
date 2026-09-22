package template

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalTemplate "github.com/0xJacky/Nginx-UI/internal/template"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pluginRoots serves one plugin template directory.
type pluginRoots struct{ root internalTemplate.Root }

func (p pluginRoots) TemplateRoots() []internalTemplate.Root { return []internalTemplate.Root{p.root} }

func TestPluginTemplatesAreServedNextToTheBuiltinOnes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "block"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "block", "hsts.conf"), []byte(`# Nginx UI Template Start
name = "Plugin HSTS"
author = "@example"
[variables.maxAge]
type = "string"
name = { en = "Max age" }
value = "600"
# Nginx UI Template End
add_header Strict-Transport-Security "max-age={{.maxAge}}" always;
`), 0o644))
	internalTemplate.RegisterSource(pluginRoots{root: internalTemplate.Root{PluginID: "io.github.example.snippets", Dir: dir}})

	router := gin.New()
	InitRouter(router.Group("/"))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/templates/blocks", nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var list struct {
		Data []internalTemplate.ConfigInfoItem `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &list))
	var builtin, plugin *internalTemplate.ConfigInfoItem
	for i, item := range list.Data {
		if item.Filename != "hsts.conf" {
			continue
		}
		if item.Origin == internalTemplate.OriginPlugin {
			plugin = &list.Data[i]
		} else {
			builtin = &list.Data[i]
		}
	}
	require.NotNil(t, builtin, "the built-in template keeps its place")
	require.NotNil(t, plugin)
	assert.Equal(t, "HSTS", builtin.Name)
	assert.Equal(t, "Plugin HSTS", plugin.Name)
	assert.Equal(t, "io.github.example.snippets", plugin.PluginID)

	// The same file name reaches the built-in template without plugin_id and
	// the plugin template with it.
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/templates/block/hsts.conf", nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Body.String(), `"origin":"builtin"`)

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/templates/block/hsts.conf?plugin_id=io.github.example.snippets",
		strings.NewReader(`{"maxAge":{"type":"string","value":"1200"}}`)))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Body.String(), `"plugin_id":"io.github.example.snippets"`)
	assert.Contains(t, recorder.Body.String(), "max-age=1200")

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/templates/block/hsts.conf?plugin_id=io.github.other", nil))
	assert.NotEqual(t, http.StatusOK, recorder.Code)
}
