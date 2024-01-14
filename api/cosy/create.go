package cosy

import (
	"github.com/gin-gonic/gin"
	"github.com/0xJacky/Nginx-UI/api/cosy/map2struct"
	"github.com/0xJacky/Nginx-UI/model"
	"gorm.io/gorm/clause"
	"net/http"
)

func (c *Ctx[T]) Create() {

	errs := c.validate()

	if len(errs) > 0 {
		c.ctx.JSON(http.StatusNotAcceptable, gin.H{
			"message": "Requested with wrong parameters",
			"errors":  errs,
		})
		return
	}

	db := model.UseDB()

	c.beforeDecodeHook()

	if c.abort {
		return
	}

	err := map2struct.WeakDecode(c.Payload, &c.Model)

	if err != nil {
		errHandler(c.ctx, err)
		return
	}

	c.beforeExecuteHook()

	if c.abort {
		return
	}

	if c.skipAssociationsOnCreate {
		err = db.Omit(clause.Associations).Create(&c.Model).Error
	} else {
		err = db.Create(&c.Model).Error
	}

	if err != nil {
		errHandler(c.ctx, err)
		return
	}

	tx := db.Preload(clause.Associations)
	for _, v := range c.preloads {
		tx = tx.Preload(v)
	}
	tx.Table(c.table, c.tableArgs...).First(&c.Model)

	if len(c.executedHookFunc) > 0 {
		for _, v := range c.executedHookFunc {
			v(c)

			if c.abort {
				return
			}
		}
	}
	if c.nextHandler != nil {
		(*c.nextHandler)(c.ctx)
	} else {
		c.ctx.JSON(http.StatusOK, c.Model)
	}
}

func (c *Ctx[T]) WithAssociations() *Ctx[T] {
	c.skipAssociationsOnCreate = false
	return c
}
