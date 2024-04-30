package cosy

import (
	"github.com/0xJacky/Nginx-UI/internal/cosy/map2struct"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
)

func (c *Ctx[T]) SetNextHandler(handler gin.HandlerFunc) *Ctx[T] {
	c.nextHandler = &handler
	return c
}

func (c *Ctx[T]) Modify() {
	if c.abort {
		return
	}
	id := c.ctx.Param("id")
	errs := c.validate()

	if len(errs) > 0 {
		c.ctx.JSON(http.StatusNotAcceptable, gin.H{
			"message": "Requested with wrong parameters",
			"errors":  errs,
		})
		return
	}

	db := model.UseDB()

	result := db
	if len(c.gormScopes) > 0 {
		result = result.Scopes(c.gormScopes...)
	}

	err := result.Session(&gorm.Session{}).First(&c.OriginModel, id).Error

	if err != nil {
		c.AbortWithError(err)
		return
	}

	c.beforeDecodeHook()
	if c.abort {
		return
	}

	var selectedFields []string

	for k := range c.Payload {
		selectedFields = append(selectedFields, k)
	}

	err = map2struct.WeakDecode(c.Payload, &c.Model)

	if err != nil {
		errHandler(c.ctx, err)
		return
	}

	c.beforeExecuteHook()
	if c.abort {
		return
	}

	if c.table != "" {
		db = db.Table(c.table, c.tableArgs...)
	}
	err = db.Model(&c.OriginModel).Select(selectedFields).Updates(&c.Model).Error

	if err != nil {
		c.AbortWithError(err)
		return
	}

	err = db.Preload(clause.Associations).First(&c.Model, id).Error

	if err != nil {
		c.AbortWithError(err)
		return
	}

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
