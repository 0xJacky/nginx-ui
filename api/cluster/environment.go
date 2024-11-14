package cluster

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/cluster"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm"
	"io"
	"net/http"
	"time"
)

func GetEnvironment(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	envQuery := query.Environment

	env, err := envQuery.FirstByID(id)
	if err != nil {
		api.ErrHandler(c, err)
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
	}).PagingList()
}

func GetAllEnabledEnvironment(c *gin.Context) {
	api.SetSSEHeaders(c)
	notify := c.Writer.CloseNotify()

	interval := 10

	type respEnvironment struct {
		*model.Environment
		Status bool `json:"status"`
	}

	f := func() (any, bool) {
		return cosy.Core[model.Environment](c).
			SetFussy("name").
			SetTransformer(func(m *model.Environment) any {
				resp := respEnvironment{
					Environment: m,
					Status:      analytic.GetNode(m).Status,
				}
				return resp
			}).ListAllData()
	}

	getHash := func(data any) string {
		bytes, _ := json.Marshal(data)
		hash := sha256.New()
		hash.Write(bytes)
		hashSum := hash.Sum(nil)
		return hex.EncodeToString(hashSum)
	}

	dataHash := ""

	{
		data, ok := f()
		if !ok {
			return
		}

		c.Stream(func(w io.Writer) bool {
			c.SSEvent("message", data)
			dataHash = getHash(data)
			return false
		})
	}

	for {
		select {
		case <-time.After(time.Duration(interval) * time.Second):
			data, ok := f()
			if !ok {
				return
			}
			// if data is not changed, send heartbeat
			if dataHash == getHash(data) {
				c.Stream(func(w io.Writer) bool {
					c.SSEvent("heartbeat", "")
					return false
				})
				return
			}

			dataHash = getHash(data)

			c.Stream(func(w io.Writer) bool {
				c.SSEvent("message", data)
				return false
			})
		case <-time.After(30 * time.Second):
			c.Stream(func(w io.Writer) bool {
				c.SSEvent("heartbeat", "")
				return false
			})
		case <-notify:
			return
		}
	}
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
	id := cast.ToUint64(c.Param("id"))
	envQuery := query.Environment

	env, err := envQuery.FirstByID(id)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	err = envQuery.DeleteByID(env.ID)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	go analytic.RestartRetrieveNodesStatus()

	c.JSON(http.StatusNoContent, nil)
}

func LoadEnvironmentFromSettings(c *gin.Context) {
	err := settings.ReloadCluster()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	cluster.RegisterPredefinedNodes()

	go analytic.RestartRetrieveNodesStatus()

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
