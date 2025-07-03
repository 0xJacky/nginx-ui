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

func GetEnvironment(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	envQuery := query.Environment

	env, err := envQuery.FirstByID(id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, analytic.GetNode(env))
}

func GetEnvironmentList(c *gin.Context) {
	core := cosy.Core[model.Environment](c).
		SetFussy("name")

	// fix for sqlite
	if c.Query("enabled") != "" {
		core.GormScope(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("enabled = ?", cast.ToInt(cast.ToBool(c.Query("enabled"))))
		})
	}

	core.SetTransformer(func(m *model.Environment) any {
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

func AddEnvironment(c *gin.Context) {
	cosy.Core[model.Environment](c).SetValidRules(gin.H{
		"name":    "required",
		"url":     "required,url",
		"token":   "required",
		"enabled": "omitempty,boolean",
	}).ExecutedHook(func(c *cosy.Ctx[model.Environment]) {
		go analytic.RestartRetrieveNodesStatus()
	}).Create()
}

func EditEnvironment(c *gin.Context) {
	cosy.Core[model.Environment](c).SetValidRules(gin.H{
		"name":    "required",
		"url":     "required,url",
		"token":   "required",
		"enabled": "omitempty,boolean",
	}).ExecutedHook(func(c *cosy.Ctx[model.Environment]) {
		go analytic.RestartRetrieveNodesStatus()
	}).Modify()
}

func DeleteEnvironment(c *gin.Context) {
	cosy.Core[model.Environment](c).
		ExecutedHook(func(c *cosy.Ctx[model.Environment]) {
			go analytic.RestartRetrieveNodesStatus()
		}).Destroy()
}

func LoadEnvironmentFromSettings(c *gin.Context) {
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
