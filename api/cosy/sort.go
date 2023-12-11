package cosy

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (c *Ctx[T]) SortOrder() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		sort := c.ctx.DefaultQuery("order", "desc")
		order := fmt.Sprintf("%s %s", DefaultQuery(c.ctx, "sort_by", c.itemKey), sort)
		return db.Order(order)
	}
}

func (c *Ctx[T]) OrderAndPaginate() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		sort := c.ctx.DefaultQuery("order", "desc")

		order := fmt.Sprintf("%s %s", DefaultQuery(c.ctx, "sort_by", c.itemKey), sort)
		db = db.Order(order)

		_, offset, pageSize := GetPagingParams(c.ctx)

		return db.Offset(offset).Limit(pageSize)
	}
}

func DefaultValue(c *gin.Context, key string, defaultValue any) any {
	if value, ok := c.Get(key); ok {
		return value
	}
	return defaultValue
}

func DefaultQuery(c *gin.Context, key string, defaultValue any) string {
	return c.DefaultQuery(key, DefaultValue(c, key, defaultValue).(string))
}
