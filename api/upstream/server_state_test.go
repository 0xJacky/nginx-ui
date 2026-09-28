package upstream

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/stream"
	internalUpstream "github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerStateRouteIsRegistered(t *testing.T) {
	router := gin.New()
	group := router.Group("/", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNoContent)
	})
	InitHTTPRouter(group)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/upstream/server_state", nil))
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// serverStateAPI extends managedAPI with the tables the site save path needs.
func serverStateAPI(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	router, confDir := managedAPI(t)
	// Registered after managedAPI's settings restore, so it runs first: the
	// background replication of site.Save / stream.Save reads those settings.
	t.Cleanup(func() {
		site.WaitForSync()
		stream.WaitForSync()
	})
	require.NoError(t, model.UseDB().AutoMigrate(&model.Site{}, &model.Stream{}, &model.Namespace{}))
	for _, dir := range []string{"sites-enabled", "streams-available", "streams-enabled"} {
		require.NoError(t, os.MkdirAll(filepath.Join(confDir, dir), 0o755))
	}
	router.POST("/upstream/server_state", SetUpstreamServerState)
	return router, confDir
}

func findExternal(t *testing.T, resp map[string]any, name string) map[string]any {
	t.Helper()
	for _, item := range resp["external"].([]any) {
		entry := item.(map[string]any)
		if entry["name"] == name {
			return entry
		}
	}
	t.Fatalf("upstream %s not listed: %v", name, resp["external"])
	return nil
}

func TestToggleSiteUpstreamServerOverHTTP(t *testing.T) {
	router, confDir := serverStateAPI(t)

	sitePath := filepath.Join(confDir, "sites-available", "shop")
	content := "upstream shop_pool {\n    server 127.0.0.1:8081;\n    server 127.0.0.1:8082;\n}\n" +
		"server { listen 80; location / { proxy_pass http://shop_pool; } }\n"
	require.NoError(t, os.WriteFile(sitePath, []byte(content), 0o644))
	require.NoError(t, os.Symlink(sitePath, filepath.Join(confDir, "sites-enabled", "shop")))
	require.NoError(t, internalUpstream.ScanConfig(sitePath, []byte(content)))

	// The list reports where the block lives and the state of every server.
	status, resp := doJSON(t, router, http.MethodGet, "/upstreams", "")
	require.Equal(t, http.StatusOK, status, resp)
	external := findExternal(t, resp, "shop_pool")
	assert.Equal(t, map[string]any{"type": "site", "name": "shop"}, external["source"])
	assert.Equal(t, false, external["read_only"])
	servers := external["servers"].([]any)
	require.Len(t, servers, 2)
	assert.Equal(t, "127.0.0.1:8082", servers[1].(map[string]any)["address"])
	assert.Equal(t, false, servers[1].(map[string]any)["down"])

	status, resp = doJSON(t, router, http.MethodPost, "/upstream/server_state",
		`{"upstream":"shop_pool","config_path":"`+sitePath+`","address":"127.0.0.1:8082","enabled":false}`)
	require.Equal(t, http.StatusOK, status, resp)
	assert.Equal(t, true, resp["servers"].([]any)[1].(map[string]any)["down"])
	written, err := os.ReadFile(sitePath)
	require.NoError(t, err)
	assert.Contains(t, string(written), "server 127.0.0.1:8082 down;")

	status, resp = doJSON(t, router, http.MethodGet, "/upstreams", "")
	require.Equal(t, http.StatusOK, status, resp)
	servers = findExternal(t, resp, "shop_pool")["servers"].([]any)
	assert.Equal(t, true, servers[1].(map[string]any)["down"])

	// A failing nginx -t is reported and leaves the file as it was.
	settings.NginxSettings.TestConfigCmd = "echo 'nginx: [emerg] broken' >&2; exit 1"
	status, resp = doJSON(t, router, http.MethodPost, "/upstream/server_state",
		`{"upstream":"shop_pool","config_path":"`+sitePath+`","address":"127.0.0.1:8082","enabled":true}`)
	assert.Equal(t, http.StatusInternalServerError, status, resp)
	assert.Contains(t, resp["params"], "nginx: [emerg] broken\n exit status 1", resp)
	after, err := os.ReadFile(sitePath)
	require.NoError(t, err)
	assert.Equal(t, string(written), string(after))
}

func TestToggleRejectsUnknownServer(t *testing.T) {
	router, confDir := serverStateAPI(t)
	sitePath := filepath.Join(confDir, "sites-available", "shop")
	content := "upstream shop_pool {\n    server 127.0.0.1:8081;\n}\n"
	require.NoError(t, os.WriteFile(sitePath, []byte(content), 0o644))
	require.NoError(t, internalUpstream.ScanConfig(sitePath, []byte(content)))

	status, resp := doJSON(t, router, http.MethodPost, "/upstream/server_state",
		`{"upstream":"shop_pool","config_path":"`+sitePath+`","address":"10.9.9.9:80","enabled":false}`)
	assert.Equal(t, http.StatusInternalServerError, status)
	requireCosyError(t, resp, 40403)
}
