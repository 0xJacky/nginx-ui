package upstream

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/testdb"
	internalUpstream "github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestManagedUpstreamRoutesAreRegistered(t *testing.T) {
	router := gin.New()
	group := router.Group("/", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNoContent)
	})
	InitHTTPRouter(group)

	for _, route := range []struct{ method, target string }{
		{http.MethodGet, "/upstreams"},
		{http.MethodGet, "/upstreams/backend_pool"},
		{http.MethodPost, "/upstreams"},
		{http.MethodPost, "/upstreams/backend_pool"},
		{http.MethodDelete, "/upstreams/backend_pool"},
		{http.MethodPost, "/upstream/preview"},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(route.method, route.target, nil))
		assert.Equal(t, http.StatusNoContent, w.Code, route.method+" "+route.target)
	}
}

// managedAPI serves the managed upstream handlers against an isolated nginx
// configuration tree whose test and reload commands always succeed.
func managedAPI(t *testing.T) (*gin.Engine, string) {
	t.Helper()

	confDir := t.TempDir()
	for _, dir := range []string{"conf.d", "sites-available"} {
		require.NoError(t, os.MkdirAll(filepath.Join(confDir, dir), 0o755))
	}
	original := *settings.NginxSettings
	settings.NginxSettings.ConfigDir = confDir
	settings.NginxSettings.PIDPath = filepath.Join(confDir, "nginx.pid")
	settings.NginxSettings.ReloadCmd = "true"
	settings.NginxSettings.TestConfigCmd = "true"
	require.NoError(t, os.WriteFile(settings.NginxSettings.PIDPath, []byte(strconv.Itoa(os.Getpid())), 0o644))
	t.Cleanup(func() { *settings.NginxSettings = original })

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Config{}, &model.ConfigBackup{}, &model.Node{}, &model.LLMSession{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	internalUpstream.GetUpstreamService().ClearTargets()
	t.Cleanup(internalUpstream.GetUpstreamService().ClearTargets)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", &model.User{Name: "admin"})
	})
	router.GET("/upstreams", ListManagedUpstreams)
	router.GET("/upstreams/:name", GetManagedUpstream)
	router.POST("/upstreams", CreateManagedUpstream)
	router.POST("/upstreams/:name", UpdateManagedUpstream)
	router.DELETE("/upstreams/:name", DeleteManagedUpstream)
	router.POST("/upstream/preview", PreviewManagedUpstream)
	return router, confDir
}

func doJSON(t *testing.T, router *gin.Engine, method, target, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), w.Body.String())
	return w.Code, resp
}

func requireCosyError(t *testing.T, resp map[string]any, code float64) {
	t.Helper()
	assert.Equal(t, "upstream", resp["scope"], resp)
	assert.Equal(t, code, resp["code"], resp)
}

func TestPreviewManagedUpstream(t *testing.T) {
	router, _ := managedAPI(t)

	status, resp := doJSON(t, router, http.MethodPost, "/upstream/preview",
		`{"name":"backend_pool","method":"least_conn","servers":[{"address":"127.0.0.1:8081"},{"address":"127.0.0.1:8082","weight":2}]}`)
	require.Equal(t, http.StatusOK, status, resp)
	assert.Equal(t, "upstream-backend_pool.conf", resp["file_name"])
	assert.Contains(t, resp["content"], "server 127.0.0.1:8082 weight=2;")

	status, resp = doJSON(t, router, http.MethodPost, "/upstream/preview",
		`{"name":"bad name","servers":[{"address":"127.0.0.1:8081"}]}`)
	assert.Equal(t, http.StatusInternalServerError, status)
	requireCosyError(t, resp, 40001)
}

func TestManagedUpstreamLifecycleOverHTTP(t *testing.T) {
	router, confDir := managedAPI(t)

	status, resp := doJSON(t, router, http.MethodPost, "/upstreams",
		`{"name":"backend_pool","method":"least_conn","servers":[{"address":"127.0.0.1:8081"},{"address":"127.0.0.1:8082"}]}`)
	require.Equal(t, http.StatusOK, status, resp)
	assert.Equal(t, filepath.Join(confDir, "conf.d", "upstream-backend_pool.conf"), resp["path"])

	// A second create with the same name is refused.
	status, resp = doJSON(t, router, http.MethodPost, "/upstreams",
		`{"name":"backend_pool","servers":[{"address":"127.0.0.1:9000"}]}`)
	assert.Equal(t, http.StatusInternalServerError, status)
	requireCosyError(t, resp, 40901)

	// The name in the path wins; the body cannot rename the group.
	status, resp = doJSON(t, router, http.MethodPost, "/upstreams/backend_pool",
		`{"name":"renamed","method":"ip_hash","servers":[{"address":"127.0.0.1:8081"}]}`)
	require.Equal(t, http.StatusOK, status, resp)
	assert.Equal(t, "backend_pool", resp["name"])
	assert.Equal(t, "ip_hash", resp["method"])
	_, err := os.Stat(filepath.Join(confDir, "conf.d", "upstream-renamed.conf"))
	assert.True(t, os.IsNotExist(err))

	// A site starts using the group: it is reported and deletion is blocked.
	require.NoError(t, os.WriteFile(filepath.Join(confDir, "sites-available", "a.example.com"),
		[]byte("server { location / { proxy_pass http://backend_pool/; } }\n"), 0o644))

	status, resp = doJSON(t, router, http.MethodGet, "/upstreams/backend_pool", "")
	require.Equal(t, http.StatusOK, status, resp)
	refs := resp["references"].([]any)
	require.Len(t, refs, 1)
	assert.Equal(t, "a.example.com", refs[0].(map[string]any)["name"])

	status, resp = doJSON(t, router, http.MethodDelete, "/upstreams/backend_pool", "")
	assert.Equal(t, http.StatusInternalServerError, status)
	requireCosyError(t, resp, 40903)
	assert.Equal(t, []any{"a.example.com"}, resp["params"])

	status, resp = doJSON(t, router, http.MethodGet, "/upstreams", "")
	require.Equal(t, http.StatusOK, status, resp)
	require.Len(t, resp["data"].([]any), 1)

	require.NoError(t, os.Remove(filepath.Join(confDir, "sites-available", "a.example.com")))
	status, resp = doJSON(t, router, http.MethodDelete, "/upstreams/backend_pool", "")
	require.Equal(t, http.StatusOK, status, resp)

	status, resp = doJSON(t, router, http.MethodGet, "/upstreams/backend_pool", "")
	assert.Equal(t, http.StatusInternalServerError, status)
	requireCosyError(t, resp, 40401)
}

func TestListManagedUpstreamsReportsExternalBlocks(t *testing.T) {
	router, confDir := managedAPI(t)

	sitePath := filepath.Join(confDir, "sites-available", "legacy")
	internalUpstream.GetUpstreamService().UpdateUpstreamDefinition("legacy_pool",
		[]internalUpstream.ProxyTarget{{Host: "10.0.0.1", Port: "80", Type: "upstream"}}, sitePath)

	status, resp := doJSON(t, router, http.MethodGet, "/upstreams", "")
	require.Equal(t, http.StatusOK, status, resp)
	assert.Empty(t, resp["data"])
	external := resp["external"].([]any)
	require.Len(t, external, 1)
	assert.Equal(t, "legacy_pool", external[0].(map[string]any)["name"])
	assert.Equal(t, sitePath, external[0].(map[string]any)["config_path"])
}

func TestCreateManagedUpstreamDefaultsToSharedZone(t *testing.T) {
	router, confDir := managedAPI(t)

	status, resp := doJSON(t, router, http.MethodPost, "/upstreams",
		`{"name":"zoned","servers":[{"address":"127.0.0.1:8081"}]}`)
	require.Equal(t, http.StatusOK, status, resp)
	assert.Equal(t, true, resp["zone"])
	assert.Equal(t, "64k", resp["zone_size"])
	content, err := os.ReadFile(filepath.Join(confDir, "conf.d", "upstream-zoned.conf"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "zone zoned 64k;")

	status, resp = doJSON(t, router, http.MethodPost, "/upstreams",
		`{"name":"plain","zone":false,"servers":[{"address":"127.0.0.1:8081"}]}`)
	require.Equal(t, http.StatusOK, status, resp)
	assert.Equal(t, false, resp["zone"])
	content, err = os.ReadFile(filepath.Join(confDir, "conf.d", "upstream-plain.conf"))
	require.NoError(t, err)
	assert.NotContains(t, string(content), "zone")

	// Updates do not force a zone onto a group that has none.
	status, resp = doJSON(t, router, http.MethodPost, "/upstreams/plain",
		`{"method":"least_conn","servers":[{"address":"127.0.0.1:8081"}]}`)
	require.Equal(t, http.StatusOK, status, resp)
	assert.Equal(t, false, resp["zone"])
}
