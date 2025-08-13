package cluster

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm"
)

type APIRespNamespace struct {
	model.Namespace
	SyncNodes []*model.Node `json:"sync_nodes,omitempty" gorm:"-"`
}

func GetNamespace(c *gin.Context) {
	cosy.Core[model.Namespace](c).
		SetTransformer(func(m *model.Namespace) any {
			db := cosy.UseDB(c)

			var nodes []*model.Node
			if len(m.SyncNodeIds) > 0 {
				db.Model(&model.Node{}).
					Where("id IN (?)", m.SyncNodeIds).
					Find(&nodes)
			}

			return &APIRespNamespace{
				Namespace: *m,
				SyncNodes: nodes,
			}
		}).
		Get()
}

func GetNamespaceList(c *gin.Context) {
	cosy.Core[model.Namespace](c).GormScope(func(tx *gorm.DB) *gorm.DB {
		return tx.Order("order_id ASC")
	}).
		SetScan(func(tx *gorm.DB) any {
			var namespaces []*APIRespNamespace

			var nodeIDs []uint64
			tx.Find(&namespaces)

			for _, namespace := range namespaces {
				nodeIDs = append(nodeIDs, namespace.SyncNodeIds...)
			}

			var nodes []*model.Node
			nodeIDs = lo.Uniq(nodeIDs)
			if len(nodeIDs) > 0 {
				db := cosy.UseDB(c)
				db.Model(&model.Node{}).
					Where("id IN (?)", nodeIDs).
					Find(&nodes)
			}

			nodeMap := lo.SliceToMap(nodes, func(node *model.Node) (uint64, *model.Node) {
				return node.ID, node
			})

			for _, namespace := range namespaces {
				for _, nodeID := range namespace.SyncNodeIds {
					if node, ok := nodeMap[nodeID]; ok {
						namespace.SyncNodes = append(namespace.SyncNodes, node)
					}
				}
			}

			return namespaces
		}).
		PagingList()
}

func AddNamespace(c *gin.Context) {
	cosy.Core[model.Namespace](c).
		SetValidRules(gin.H{
			"name":               "required",
			"sync_node_ids":      "omitempty",
			"post_sync_action":   "omitempty,oneof=" + model.PostSyncActionNone + " " + model.PostSyncActionReloadNginx,
			"upstream_test_type": "omitempty,oneof=" + model.UpstreamTestLocal + " " + model.UpstreamTestRemote + " " + model.UpstreamTestMirror,
		}).
		Create()
}

func ModifyNamespace(c *gin.Context) {
	cosy.Core[model.Namespace](c).
		SetValidRules(gin.H{
			"name":               "required",
			"sync_node_ids":      "omitempty",
			"post_sync_action":   "omitempty,oneof=" + model.PostSyncActionNone + " " + model.PostSyncActionReloadNginx,
			"upstream_test_type": "omitempty,oneof=" + model.UpstreamTestLocal + " " + model.UpstreamTestRemote + " " + model.UpstreamTestMirror,
		}).
		Modify()
}

func DeleteNamespace(c *gin.Context) {
	cosy.Core[model.Namespace](c).Destroy()
}

func RecoverNamespace(c *gin.Context) {
	cosy.Core[model.Namespace](c).Recover()
}

func UpdateNamespacesOrder(c *gin.Context) {
	cosy.Core[model.Namespace](c).UpdateOrder()
}

func ReloadNginx(c *gin.Context) {
	var json struct {
		NodeIDs []uint64 `json:"node_ids" binding:"required"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	go syncReload(json.NodeIDs)

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func RestartNginx(c *gin.Context) {
	var json struct {
		NodeIDs []uint64 `json:"node_ids" binding:"required"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	go syncRestart(json.NodeIDs)

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}