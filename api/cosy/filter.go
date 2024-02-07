package cosy

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
)

func (c *Ctx[T]) SetFussy(keys ...string) *Ctx[T] {
	c.gormScopes = append(c.gormScopes, func(tx *gorm.DB) *gorm.DB {
		return QueryToFussySearch(c.ctx, tx, keys...)
	})
	return c
}

func (c *Ctx[T]) SetFussyKeys(value string, keys ...string) *Ctx[T] {
	c.gormScopes = append(c.gormScopes, func(tx *gorm.DB) *gorm.DB {
		return QueryToFussyKeysSearch(c.ctx, tx, value, keys...)
	})
	return c
}

func (c *Ctx[T]) SetEqual(keys ...string) *Ctx[T] {
	c.gormScopes = append(c.gormScopes, func(tx *gorm.DB) *gorm.DB {
		return QueryToEqualSearch(c.ctx, tx, keys...)
	})
	return c
}

func (c *Ctx[T]) SetIn(keys ...string) *Ctx[T] {
	c.gormScopes = append(c.gormScopes, func(tx *gorm.DB) *gorm.DB {
		return QueryToInSearch(c.ctx, tx, keys...)
	})
	return c
}

func (c *Ctx[T]) SetOrFussy(keys ...string) *Ctx[T] {
	c.gormScopes = append(c.gormScopes, func(tx *gorm.DB) *gorm.DB {
		return QueryToOrFussySearch(c.ctx, tx, keys...)
	})
	return c
}

func (c *Ctx[T]) SetOrEqual(keys ...string) *Ctx[T] {
	c.gormScopes = append(c.gormScopes, func(tx *gorm.DB) *gorm.DB {
		return QueryToOrEqualSearch(c.ctx, tx, keys...)
	})
	return c
}

func (c *Ctx[T]) SetOrIn(keys ...string) *Ctx[T] {
	c.gormScopes = append(c.gormScopes, func(tx *gorm.DB) *gorm.DB {
		return QueryToOrInSearch(c.ctx, tx, keys...)
	})
	return c
}

func QueryToInSearch(c *gin.Context, db *gorm.DB, keys ...string) *gorm.DB {
	for _, v := range keys {
		queryArray := c.QueryArray(v + "[]")
		if len(queryArray) == 0 {
			queryArray = c.QueryArray(v)
		}
		if len(queryArray) == 1 && queryArray[0] == "" {
			continue
		}
		if len(queryArray) >= 1 {
			var builder strings.Builder
			stmt := db.Statement

			stmt.QuoteTo(&builder, clause.Column{Table: stmt.Table, Name: v})
			builder.WriteString(" IN ?")

			db = db.Where(builder.String(), queryArray)
		}
	}
	return db
}

func QueryToEqualSearch(c *gin.Context, db *gorm.DB, keys ...string) *gorm.DB {
	for _, v := range keys {
		if c.Query(v) != "" {
			var sb strings.Builder

			_, err := fmt.Fprintf(&sb, "`%s` = ?", v)
			if err != nil {
				logger.Error(err)
				continue
			}

			db = db.Where(sb.String(), c.Query(v))
		}
	}
	return db
}

func QueryToFussySearch(c *gin.Context, db *gorm.DB, keys ...string) *gorm.DB {
	for _, v := range keys {
		if c.Query(v) != "" {
			var sb strings.Builder

			_, err := fmt.Fprintf(&sb, "`%s` LIKE ?", v)
			if err != nil {
				logger.Error(err)
				continue
			}

			var sbValue strings.Builder

			_, err = fmt.Fprintf(&sbValue, "%%%s%%", c.Query(v))

			if err != nil {
				logger.Error(err)
				continue
			}

			db = db.Where(sb.String(), sbValue.String())
		}
	}
	return db
}

func QueryToFussyKeysSearch(c *gin.Context, db *gorm.DB, value string, keys ...string) *gorm.DB {
	if c.Query(value) == "" {
		return db
	}

	var condition *gorm.DB
	for i, v := range keys {
		sb := v + " LIKE ?"
		sv := "%" + c.Query(value) + "%"

		switch i {
		case 0:
			condition = db.Where(db.Where(sb, sv))
		default:
			condition = condition.Or(sb, sv)
		}
	}

	return db.Where(condition)
}

func QueryToOrInSearch(c *gin.Context, db *gorm.DB, keys ...string) *gorm.DB {
	for _, v := range keys {
		queryArray := c.QueryArray(v + "[]")
		if len(queryArray) == 0 {
			queryArray = c.QueryArray(v)
		}
		if len(queryArray) == 1 && queryArray[0] == "" {
			continue
		}
		if len(queryArray) >= 1 {
			var sb strings.Builder

			_, err := fmt.Fprintf(&sb, "`%s` IN ?", v)
			if err != nil {
				logger.Error(err)
				continue
			}

			db = db.Or(sb.String(), queryArray)
		}
	}
	return db
}

func QueryToOrEqualSearch(c *gin.Context, db *gorm.DB, keys ...string) *gorm.DB {
	for _, v := range keys {
		if c.Query(v) != "" {
			var sb strings.Builder

			_, err := fmt.Fprintf(&sb, "`%s` = ?", v)
			if err != nil {
				logger.Error(err)
				continue
			}

			db = db.Or(sb.String(), c.Query(v))
		}
	}
	return db
}

func QueryToOrFussySearch(c *gin.Context, db *gorm.DB, keys ...string) *gorm.DB {
	for _, v := range keys {
		if c.Query(v) != "" {
			var sb strings.Builder

			_, err := fmt.Fprintf(&sb, "`%s` LIKE ?", v)
			if err != nil {
				logger.Error(err)
				continue
			}

			var sbValue strings.Builder

			_, err = fmt.Fprintf(&sbValue, "%%%s%%", c.Query(v))

			if err != nil {
				logger.Error(err)
				continue
			}

			db = db.Or(sb.String(), sbValue.String())
		}
	}
	return db
}
