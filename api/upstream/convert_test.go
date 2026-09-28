package upstream

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	internalUpstream "github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/internal/upstream/convert"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertRouteIsRegistered(t *testing.T) {
	router := gin.New()
	group := router.Group("/", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNoContent)
	})
	InitHTTPRouter(group)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/upstream/convert", nil))
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestConvertSiteUpstreamOverHTTP(t *testing.T) {
	router, confDir := serverStateAPI(t)
	t.Cleanup(convert.WaitForSync)
	router.POST("/upstream/convert", ConvertSiteUpstream)

	sitePath := filepath.Join(confDir, "sites-available", "shop")
	content := "upstream shop_pool {\n    least_conn;\n    server 127.0.0.1:8081;\n    server 127.0.0.1:8082;\n}\n\n" +
		"server { listen 80; location / { proxy_pass http://shop_pool; } }\n"
	require.NoError(t, os.WriteFile(sitePath, []byte(content), 0o644))
	require.NoError(t, os.Symlink(sitePath, filepath.Join(confDir, "sites-enabled", "shop")))
	require.NoError(t, internalUpstream.ScanConfig(sitePath, []byte(content)))

	status, resp := doJSON(t, router, http.MethodPost, "/upstream/convert", `{"site":"shop","upstream":"shop_pool"}`)
	require.Equal(t, http.StatusOK, status, resp)
	assert.Equal(t, "shop_pool", resp["name"])
	assert.Equal(t, "least_conn", resp["method"])

	written, err := os.ReadFile(sitePath)
	require.NoError(t, err)
	assert.Equal(t, "server { listen 80; location / { proxy_pass http://shop_pool; } }\n", string(written))

	// The group is listed as managed and no longer as a block of the site.
	status, resp = doJSON(t, router, http.MethodGet, "/upstreams", "")
	require.Equal(t, http.StatusOK, status, resp)
	data := resp["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, "shop_pool", data[0].(map[string]any)["name"])
	for _, item := range resp["external"].([]any) {
		assert.NotEqual(t, "shop_pool", item.(map[string]any)["name"])
	}

	// Converting again is refused: the group exists now and the block is gone.
	status, resp = doJSON(t, router, http.MethodPost, "/upstream/convert", `{"site":"shop","upstream":"shop_pool"}`)
	assert.Equal(t, http.StatusInternalServerError, status)
	requireCosyError(t, resp, 40906)
}
