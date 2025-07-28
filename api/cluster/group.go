package cluster

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm"
)

type APIRespEnvGroup struct {
	model.EnvGroup
	SyncNodes []*model.Environment `json:"sync_nodes,omitempty" gorm:"-"`
}

func GetGroup(c *gin.Context) {
	cosy.Core[model.EnvGroup](c).
		SetTransformer(func(m *model.EnvGroup) any {
			db := cosy.UseDB(c)

			var nodes []*model.Environment
			if len(m.SyncNodeIds) > 0 {
				db.Model(&model.Environment{}).
					Where("id IN (?)", m.SyncNodeIds).
					Find(&nodes)
			}

			return &APIRespEnvGroup{
				EnvGroup:  *m,
				SyncNodes: nodes,
			}
		}).
		Get()
}

func GetGroupList(c *gin.Context) {
	cosy.Core[model.EnvGroup](c).GormScope(func(tx *gorm.DB) *gorm.DB {
		return tx.Order("order_id ASC")
	}).
		SetScan(func(tx *gorm.DB) any {
			var groups []*APIRespEnvGroup

			var nodeIDs []uint64
			tx.Find(&groups)

			for _, group := range groups {
				nodeIDs = append(nodeIDs, group.SyncNodeIds...)
			}

			var nodes []*model.Environment
			nodeIDs = lo.Uniq(nodeIDs)
			if len(nodeIDs) > 0 {
				db := cosy.UseDB(c)
				db.Model(&model.Environment{}).
					Where("id IN (?)", nodeIDs).
					Find(&nodes)
			}

			nodeMap := lo.SliceToMap(nodes, func(node *model.Environment) (uint64, *model.Environment) {
				return node.ID, node
			})

			for _, group := range groups {
				for _, nodeID := range group.SyncNodeIds {
					if node, ok := nodeMap[nodeID]; ok {
						group.SyncNodes = append(group.SyncNodes, node)
					}
				}
			}

			return groups
		}).
		PagingList()
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

func AddGroup(c *gin.Context) {
	cosy.Core[model.EnvGroup](c).
		SetValidRules(gin.H{
			"name":             "required",
			"sync_node_ids":    "omitempty",
			"post_sync_action": "omitempty,oneof=" + model.PostSyncActionNone + " " + model.PostSyncActionReloadNginx,
		}).
		Create()
}

func ModifyGroup(c *gin.Context) {
	cosy.Core[model.EnvGroup](c).
		SetValidRules(gin.H{
			"name":             "required",
			"sync_node_ids":    "omitempty",
			"post_sync_action": "omitempty,oneof=" + model.PostSyncActionNone + " " + model.PostSyncActionReloadNginx,
		}).
		Modify()
}

func DeleteGroup(c *gin.Context) {
	cosy.Core[model.EnvGroup](c).Destroy()
}

func RecoverGroup(c *gin.Context) {
	cosy.Core[model.EnvGroup](c).Recover()
}

func UpdateGroupsOrder(c *gin.Context) {
	cosy.Core[model.EnvGroup](c).UpdateOrder()
}
