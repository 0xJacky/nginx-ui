package cluster

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/clustersync"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"gorm.io/gen/field"
)

// SyncNodes pushes the local configurations, sites and streams to the selected
// nodes. A freshly added node is brought up to date with a single request.
func SyncNodes(c *gin.Context) {
	var json struct {
		NodeIDs   []uint64 `json:"node_ids" binding:"required"`
		Configs   *bool    `json:"configs"`
		Sites     *bool    `json:"sites"`
		Streams   *bool    `json:"streams"`
		Overwrite *bool    `json:"overwrite"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	scope := clustersync.FullScope()
	if json.Configs != nil {
		scope.Configs = *json.Configs
	}
	if json.Sites != nil {
		scope.Sites = *json.Sites
	}
	if json.Streams != nil {
		scope.Streams = *json.Streams
	}
	if json.Overwrite != nil {
		scope.Overwrite = *json.Overwrite
	}

	summary, err := clustersync.SyncNodes(c, json.NodeIDs, scope)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, summary)
}

// SyncNamespace replicates a namespace and all of its sites and streams to every
// node of that namespace, so each node ends up with the same content.
func SyncNamespace(c *gin.Context) {
	summary, err := clustersync.SyncNamespace(c, cast.ToUint64(c.Param("id")))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, summary)
}

// UpsertNamespace is the receiving end of namespace replication. It creates the
// namespace when the node does not know it yet and keeps its local node
// membership and deploy mode untouched.
func UpsertNamespace(c *gin.Context) {
	var json struct {
		Name             string `json:"name" binding:"required"`
		PostSyncAction   string `json:"post_sync_action" binding:"omitempty,oneof=none reload_nginx"`
		UpstreamTestType string `json:"upstream_test_type" binding:"omitempty,oneof=local remote mirror"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	n := query.Namespace
	namespace, err := n.Assign(field.Attrs(&model.Namespace{
		Name:             json.Name,
		PostSyncAction:   json.PostSyncAction,
		UpstreamTestType: json.UpstreamTestType,
	})).Where(n.Name.Eq(json.Name)).FirstOrCreate()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, namespace)
}
