package cluster

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

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

const reloadDispatchCooldown = 2 * time.Second

type reloadDispatchGate struct {
	mutex        sync.Mutex
	inFlight     bool
	lastFinished time.Time
	cooldown     time.Duration
}

type reloadDispatchDependencies struct {
	now      func() time.Time
	dispatch func(nodeIDs []uint64)
}

var clusterReloadDispatchGate = &reloadDispatchGate{cooldown: reloadDispatchCooldown}

func (gate *reloadDispatchGate) tryStart(now time.Time) (ok bool, retryAfter time.Duration) {
	gate.mutex.Lock()
	defer gate.mutex.Unlock()

	if gate.inFlight {
		return false, 0
	}
	if !gate.lastFinished.IsZero() {
		retryAfter = gate.cooldown - now.Sub(gate.lastFinished)
		if retryAfter > 0 {
			return false, retryAfter
		}
	}

	gate.inFlight = true
	return true, 0
}

func (gate *reloadDispatchGate) finish(now time.Time) {
	gate.mutex.Lock()
	gate.inFlight = false
	gate.lastFinished = now
	gate.mutex.Unlock()
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
			"name":                  "required",
			"sync_node_ids":         "omitempty",
			"post_sync_action":      "omitempty,oneof=" + model.PostSyncActionNone + " " + model.PostSyncActionReloadNginx,
			"upstream_test_type":    "omitempty,oneof=" + model.UpstreamTestLocal + " " + model.UpstreamTestRemote + " " + model.UpstreamTestMirror,
			"deploy_mode":           "omitempty,oneof=" + model.DeployModeLocal + " " + model.DeployModeRemote,
			"sync_strategy":         "omitempty,oneof=" + model.SyncStrategyManual + " " + model.SyncStrategyAuto,
			"sync_interval_minutes": "omitempty,min=0",
		}).
		Create()
}

func ModifyNamespace(c *gin.Context) {
	cosy.Core[model.Namespace](c).
		SetValidRules(gin.H{
			"name":                  "required",
			"sync_node_ids":         "omitempty",
			"post_sync_action":      "omitempty,oneof=" + model.PostSyncActionNone + " " + model.PostSyncActionReloadNginx,
			"upstream_test_type":    "omitempty,oneof=" + model.UpstreamTestLocal + " " + model.UpstreamTestRemote + " " + model.UpstreamTestMirror,
			"deploy_mode":           "omitempty,oneof=" + model.DeployModeLocal + " " + model.DeployModeRemote,
			"sync_strategy":         "omitempty,oneof=" + model.SyncStrategyManual + " " + model.SyncStrategyAuto,
			"sync_interval_minutes": "omitempty,min=0",
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
	reloadNginx(c, reloadDispatchDependencies{
		now:      time.Now,
		dispatch: syncReload,
	}, clusterReloadDispatchGate)
}

func reloadNginx(c *gin.Context, dependencies reloadDispatchDependencies, gate *reloadDispatchGate) {
	var json struct {
		NodeIDs []uint64 `json:"node_ids" binding:"required"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	started, retryAfter := gate.tryStart(dependencies.now())
	if !started {
		if retryAfter > 0 {
			seconds := int(math.Ceil(retryAfter.Seconds()))
			c.Header("Retry-After", strconv.Itoa(seconds))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"message":     "Nginx reload dispatch is cooling down",
				"retry_after": seconds,
			})
			return
		}
		c.JSON(http.StatusConflict, gin.H{
			"message": "another Nginx reload dispatch is already running",
		})
		return
	}

	go func() {
		defer gate.finish(dependencies.now())
		dependencies.dispatch(json.NodeIDs)
	}()

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
