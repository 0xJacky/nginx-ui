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
	// Without a zone field the group gets the default shared memory zone.
	assert.Equal(t, true, resp["zone"])
	assert.Equal(t, "64k", resp["zone_size"])

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

// The convert dialog shows the zone a block already declares instead of the
// zone switch, so the list reports it.
func TestListReportsZoneOfExternalBlock(t *testing.T) {
	router, confDir := serverStateAPI(t)

	sitePath := filepath.Join(confDir, "sites-available", "shop")
	content := "upstream shop_pool {\n    zone shop_pool 128k;\n    server 127.0.0.1:8081;\n}\n" +
		"upstream plain_pool {\n    server 127.0.0.1:8082;\n}\n"
	require.NoError(t, os.WriteFile(sitePath, []byte(content), 0o644))
	require.NoError(t, internalUpstream.ScanConfig(sitePath, []byte(content)))

	status, resp := doJSON(t, router, http.MethodGet, "/upstreams", "")
	require.Equal(t, http.StatusOK, status, resp)
	zones := map[string]any{}
	for _, item := range resp["external"].([]any) {
		entry := item.(map[string]any)
		zones[entry["name"].(string)] = entry["zone"]
	}
	assert.Equal(t, map[string]any{"shop_pool": "shop_pool 128k", "plain_pool": nil}, zones)
}

func TestConvertSiteUpstreamHonorsZoneFields(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantZone string
	}{
		{name: "zone off", body: `{"site":"shop","upstream":"shop_pool","zone":false}`},
		{name: "custom size", body: `{"site":"shop","upstream":"shop_pool","zone":true,"zone_size":"256k"}`, wantZone: "    zone shop_pool 256k;\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, confDir := serverStateAPI(t)
			t.Cleanup(convert.WaitForSync)
			router.POST("/upstream/convert", ConvertSiteUpstream)

			sitePath := filepath.Join(confDir, "sites-available", "shop")
			content := "upstream shop_pool {\n    server 127.0.0.1:8081;\n}\n\n" +
				"server { listen 80; location / { proxy_pass http://shop_pool; } }\n"
			require.NoError(t, os.WriteFile(sitePath, []byte(content), 0o644))
			require.NoError(t, internalUpstream.ScanConfig(sitePath, []byte(content)))

			status, resp := doJSON(t, router, http.MethodPost, "/upstream/convert", tc.body)
			require.Equal(t, http.StatusOK, status, resp)
			assert.Equal(t, tc.wantZone != "", resp["zone"])

			group, err := os.ReadFile(filepath.Join(confDir, "conf.d", "upstream-shop_pool.conf"))
			require.NoError(t, err)
			if tc.wantZone == "" {
				assert.NotContains(t, string(group), "zone")
			} else {
				assert.Contains(t, string(group), tc.wantZone)
			}
		})
	}
}
