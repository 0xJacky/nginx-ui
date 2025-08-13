package cluster

import (
	"context"
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/cluster"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm"
)

func GetNode(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	nodeQuery := query.Node

	node, err := nodeQuery.FirstByID(id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, analytic.GetNode(node))
}

func GetNodeList(c *gin.Context) {
	core := cosy.Core[model.Node](c).
		SetFussy("name")

	// fix for sqlite
	if c.Query("enabled") != "" {
		core.GormScope(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("enabled = ?", cast.ToInt(cast.ToBool(c.Query("enabled"))))
		})
	}

	core.SetTransformer(func(m *model.Node) any {
		return analytic.GetNode(m)
	})

	data, ok := core.ListAllData()
	if !ok {
		return
	}

	c.JSON(http.StatusOK, model.DataList{
		Data: data,
	})
}

func AddNode(c *gin.Context) {
	cosy.Core[model.Node](c).SetValidRules(gin.H{
		"name":    "required",
		"url":     "required,url",
		"token":   "required",
		"enabled": "omitempty,boolean",
	}).ExecutedHook(func(c *cosy.Ctx[model.Node]) {
		go analytic.RestartRetrieveNodesStatus()
	}).Create()
}

func EditNode(c *gin.Context) {
	cosy.Core[model.Node](c).SetValidRules(gin.H{
		"name":    "required",
		"url":     "required,url",
		"token":   "required",
		"enabled": "omitempty,boolean",
	}).ExecutedHook(func(c *cosy.Ctx[model.Node]) {
		go analytic.RestartRetrieveNodesStatus()
	}).Modify()
}

func DeleteNode(c *gin.Context) {
	cosy.Core[model.Node](c).
		ExecutedHook(func(c *cosy.Ctx[model.Node]) {
			go analytic.RestartRetrieveNodesStatus()
		}).Destroy()
}

func LoadNodeFromSettings(c *gin.Context) {
	err := settings.ReloadCluster()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	ctx := context.Background()
	cluster.RegisterPredefinedNodes(ctx)

	go analytic.RestartRetrieveNodesStatus()

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
