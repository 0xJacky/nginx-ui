package cluster

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm"
)

func GetGroup(c *gin.Context) {
	cosy.Core[model.EnvGroup](c).Get()
}

func GetGroupList(c *gin.Context) {
	cosy.Core[model.EnvGroup](c).GormScope(func(tx *gorm.DB) *gorm.DB {
		return tx.Order("order_id ASC")
	}).PagingList()
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
			"name":          "required",
			"sync_node_ids": "omitempty",
		}).
		Create()
}

func ModifyGroup(c *gin.Context) {
	cosy.Core[model.EnvGroup](c).
		SetValidRules(gin.H{
			"name":          "required",
			"sync_node_ids": "omitempty",
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
