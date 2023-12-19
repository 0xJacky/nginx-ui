package cosy

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"sync"
)

func (c *Ctx[T]) SortOrder() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		sort := c.ctx.DefaultQuery("order", "desc")
		if sort != "desc" && sort != "asc" {
			sort = "desc"
		}

		// check if the order field is valid
		// todo: maybe we can use more generic way to check if the sort_by is valid
		order := DefaultQuery(c.ctx, "sort_by", c.itemKey)
		s, _ := schema.Parse(c.Model, &sync.Map{}, schema.NamingStrategy{})
		if _, ok := s.FieldsByDBName[order]; ok {
			order = fmt.Sprintf("%s %s", order, sort)
			return db.Order(order)
		} else {
			logger.Error("invalid order field:", order)
		}

		return db
	}
}

func (c *Ctx[T]) OrderAndPaginate() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		db = c.SortOrder()(db)
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
