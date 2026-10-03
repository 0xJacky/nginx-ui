package plugin

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/middleware"
	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// syncResponse is the body of the sync endpoint.
type syncResponse struct {
	Results []plugin.NodeResult `json:"results"`
}

// InitSyncRouter registers the plugin cluster sync API. Pushing a plugin to a
// node installs code there, so the mutations are held to the same bar as a
// local install.
func InitSyncRouter(r *gin.RouterGroup) {
	r.GET("/plugins/matrix", GetPluginMatrix)

	o := r.Group("", middleware.RequireSecureSession(), middleware.RejectInDemo())
	{
		o.POST("/plugins/:id/sync", SyncPluginToNodes)
		o.POST("/plugins/:id/sync_policy", SetPluginSyncPolicy)
	}
}

// GetPluginMatrix reports every installed plugin against every enabled node.
func GetPluginMatrix(c *gin.Context) {
	matrix, err := plugin.GetManager().Syncer().Matrix(c)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, matrix)
}

// SyncPluginToNodes pushes one plugin to the given nodes. An empty node list
// falls back to the target set stored on the plugin.
func SyncPluginToNodes(c *gin.Context) {
	id, ok := pluginID(c)
	if !ok {
		return
	}

	var body struct {
		NodeIDs []uint64 `json:"node_ids"`
	}
	// The UI sends an empty body when it syncs the configured target set.
	_ = c.ShouldBindJSON(&body)

	results, err := plugin.GetManager().Syncer().SyncPlugin(detach(c), id, body.NodeIDs)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if results == nil {
		results = []plugin.NodeResult{}
	}
	c.JSON(http.StatusOK, syncResponse{Results: results})
}

// SetPluginSyncPolicy stores whether a plugin is kept in sync automatically.
func SetPluginSyncPolicy(c *gin.Context) {
	id, ok := pluginID(c)
	if !ok {
		return
	}

	var body struct {
		SyncPolicy   string   `json:"sync_policy" binding:"omitempty,oneof=manual auto"`
		SyncNodeIDs  []uint64 `json:"sync_node_ids"`
		SyncSettings bool     `json:"sync_settings"`
	}
	if !cosy.BindAndValid(c, &body) {
		return
	}

	manager := plugin.GetManager()
	if err := manager.Syncer().SetPolicy(detach(c), id, body.SyncPolicy, body.SyncNodeIDs, body.SyncSettings); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	info, err := manager.Get(id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}
