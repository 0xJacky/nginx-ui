package cosy

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/logger"
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

		order := c.itemKey
		if value, ok := c.ctx.Get("order"); ok {
			// check if the order field is valid
			// todo: maybe we can use more generic way to check if the sort_by is valid
			s, _ := schema.Parse(c.Model, &sync.Map{}, schema.NamingStrategy{})
			if _, ok := s.FieldsByDBName[value.(string)]; ok {
				order = value.(string)
			} else {
				logger.Error("invalid order field:", order)
			}
		} else if value, ok := c.ctx.Get("sort_by"); ok {
			order = value.(string)
		}

		order = fmt.Sprintf("%s %s", order, sort)
		return db.Order(order)
	}
}

func (c *Ctx[T]) OrderAndPaginate() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		db = c.SortOrder()(db)
		_, offset, pageSize := GetPagingParams(c.ctx)
		return db.Offset(offset).Limit(pageSize)
	}
}
