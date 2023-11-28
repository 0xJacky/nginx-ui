package cosy

import (
	"github.com/0xJacky/Nginx-UI/model"
	"gorm.io/gorm"
	"net/http"
)

func (c *Ctx[T]) Destroy() {
	if c.abort {
		return
	}
	id := c.ctx.Param("id")

	c.beforeExecuteHook()

	db := model.UseDB()
	var dbModel T

	result := db
	if len(c.gormScopes) > 0 {
		result = result.Scopes(c.gormScopes...)
	}

	err := result.Session(&gorm.Session{}).First(&dbModel, id).Error

	if err != nil {
		errHandler(c.ctx, err)
		return
	}

	err = result.Delete(&dbModel).Error
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

	err := result.Session(&gorm.Session{}).First(&dbModel, id).Error

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
