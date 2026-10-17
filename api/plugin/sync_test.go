package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupSyncManager adds the node table the cluster sync handlers read.
func setupSyncManager(t *testing.T) *plugin.Manager {
	t.Helper()
	manager := setupManager(t)
	require.NoError(t, model.UseDB().AutoMigrate(&model.Node{}))
	return manager
}

// TestSyncRouterCoexistsWithThePluginRoutes guards the static "matrix" segment
// against the ":id" parameter the other plugin routes use.
func TestSyncRouterCoexistsWithThePluginRoutes(t *testing.T) {
	engine := gin.New()
	group := engine.Group("/api")
	require.NotPanics(t, func() {
		InitRouter(group)
		InitSyncRouter(group)
	})

	paths := map[string]bool{}
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	assert.True(t, paths["GET /api/plugins/matrix"])
	assert.True(t, paths["POST /api/plugins/:id/sync"])
	assert.True(t, paths["POST /api/plugins/:id/sync_policy"])
}

func TestGetPluginMatrixListsEveryPluginWithoutNodes(t *testing.T) {
	manager := setupSyncManager(t)
	installTestPlugin(t, manager, webappManifest("official.alpha"), nil, true)

	c, recorder := newContext(http.MethodGet, "/api/plugins/matrix", nil, nil)
	GetPluginMatrix(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var matrix plugin.Matrix
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &matrix))
	assert.Empty(t, matrix.Nodes)
	require.Len(t, matrix.Rows, 1)
	assert.Equal(t, "official.alpha", matrix.Rows[0].PluginID)
	assert.Equal(t, model.PluginSyncPolicyManual, matrix.Rows[0].SyncPolicy)
	assert.Empty(t, matrix.Rows[0].Cells)
}

func TestSetPluginSyncPolicyPersistsTheIntent(t *testing.T) {
	manager := setupSyncManager(t)
	installTestPlugin(t, manager, webappManifest("official.alpha"), nil, true)

	payload := `{"sync_policy":"auto","sync_node_ids":[3,5],"sync_settings":true}`
	c, recorder := newContext(http.MethodPost, "/api/plugins/official.alpha/sync_policy",
		strings.NewReader(payload), gin.Params{{Key: "id", Value: "official.alpha"}})
	SetPluginSyncPolicy(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var info plugin.Info
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &info))
	assert.Equal(t, model.PluginSyncPolicyAuto, info.SyncPolicy)
	assert.Equal(t, []uint64{3, 5}, info.SyncNodeIDs)
	assert.True(t, info.SyncSettings)

	stored, err := query.Plugin.WithContext(context.Background()).
		Where(query.Plugin.PluginID.Eq("official.alpha")).First()
	require.NoError(t, err)
	assert.Equal(t, model.PluginSyncPolicyAuto, stored.SyncPolicy)
	assert.Equal(t, []uint64{3, 5}, stored.SyncNodeIDs)
	assert.True(t, stored.SyncSettings)
}

func TestSetPluginSyncPolicyRejectsAnUnknownPolicy(t *testing.T) {
	manager := setupSyncManager(t)
	installTestPlugin(t, manager, webappManifest("official.alpha"), nil, true)

	c, recorder := newContext(http.MethodPost, "/api/plugins/official.alpha/sync_policy",
		strings.NewReader(`{"sync_policy":"nonsense"}`), gin.Params{{Key: "id", Value: "official.alpha"}})
	SetPluginSyncPolicy(c)

	assert.NotEqual(t, http.StatusOK, recorder.Code)
}

func TestSyncPluginToNodesWithoutNodesReturnsNoResults(t *testing.T) {
	manager := setupSyncManager(t)
	installTestPlugin(t, manager, webappManifest("official.alpha"), nil, true)

	c, recorder := newContext(http.MethodPost, "/api/plugins/official.alpha/sync", nil,
		gin.Params{{Key: "id", Value: "official.alpha"}})
	SyncPluginToNodes(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Results []plugin.NodeResult `json:"results"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.NotNil(t, body.Results)
	assert.Empty(t, body.Results)
}

func TestSyncHandlersRejectMalformedPluginIDs(t *testing.T) {
	setupSyncManager(t)

	c, recorder := newContext(http.MethodPost, "/api/plugins/..%2f..%2fetc/sync", nil,
		gin.Params{{Key: "id", Value: "../../etc"}})
	SyncPluginToNodes(c)
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "55002")

	c, recorder = newContext(http.MethodPost, "/api/plugins/..%2f..%2fetc/sync_policy",
		strings.NewReader(`{"sync_policy":"auto"}`), gin.Params{{Key: "id", Value: "../../etc"}})
	SetPluginSyncPolicy(c)
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "55002")
}
