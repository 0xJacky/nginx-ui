package cosy

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"strings"
	"sync"
)

func (c *Ctx[T]) SortOrder() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		order := c.ctx.DefaultQuery("order", "desc")
		if order != "desc" && order != "asc" {
			order = "desc"
		}

		sortBy := c.ctx.DefaultQuery("sort_by", c.itemKey)

		s, _ := schema.Parse(c.Model, &sync.Map{}, schema.NamingStrategy{})
		if _, ok := s.FieldsByDBName[sortBy]; !ok && sortBy != c.itemKey {
			logger.Error("invalid order field:", sortBy)
			return db
		}

		var sb strings.Builder
		sb.WriteString(sortBy)
		sb.WriteString(" ")
		sb.WriteString(order)

		return db.Order(sb.String())
	}
}

func (c *Ctx[T]) OrderAndPaginate() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		db = c.SortOrder()(db)
		_, offset, pageSize := GetPagingParams(c.ctx)
		return db.Offset(offset).Limit(pageSize)
	}
}
