package cosy

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/spf13/cast"
	"gorm.io/gorm"
	"net/http"
)

func (c *Ctx[T]) PermanentlyDelete() {
	c.permanentlyDelete = true
	c.Destroy()
}

func (c *Ctx[T]) Destroy() {
	if c.abort {
		return
	}
	id := c.ctx.Param("id")

	c.beforeExecuteHook()

	db := model.UseDB()

	result := db

	if cast.ToBool(c.ctx.Query("permanent")) || c.permanentlyDelete {
		result = result.Unscoped()
	}

	if len(c.gormScopes) > 0 {
		result = result.Scopes(c.gormScopes...)
	}

	var err error
	session := result.Session(&gorm.Session{})
	if c.table != "" {
		err = session.Table(c.table, c.tableArgs...).Take(c.OriginModel, id).Error
	} else {
		err = session.First(&c.OriginModel, id).Error
	}

	if err != nil {
		errHandler(c.ctx, err)
		return
	}

	err = result.Delete(&c.OriginModel).Error
	if err != nil {
		errHandler(c.ctx, err)
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

	c.ctx.JSON(http.StatusNoContent, nil)
}

func (c *Ctx[T]) Recover() {
	if c.abort {
		return
	}
	id := c.ctx.Param("id")

	c.beforeExecuteHook()

	db := model.UseDB()
	var dbModel T

	result := db.Unscoped()
	if len(c.gormScopes) > 0 {
		result = result.Scopes(c.gormScopes...)
	}

	var err error
	session := result.Session(&gorm.Session{})
	if c.table != "" {
		err = session.Table(c.table).Take(&dbModel, id).Error
	} else {
		err = session.First(&dbModel, id).Error
	}

	if err != nil {
		errHandler(c.ctx, err)
		return
	}

	err = result.Model(&dbModel).Update("deleted_at", nil).Error
	if err != nil {
		errHandler(c.ctx, err)
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

	c.ctx.JSON(http.StatusNoContent, nil)
}
