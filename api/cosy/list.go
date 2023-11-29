package cosy

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/spf13/cast"
	"gorm.io/gorm"
	"net/http"
)

func (c *Ctx[T]) SetFussy(keys ...string) *Ctx[T] {
	c.gormScopes = append(c.gormScopes, func(tx *gorm.DB) *gorm.DB {
		return model.QueryToFussySearch(c.ctx, tx, keys...)
	})
	return c
}

func (c *Ctx[T]) SetFussyKeys(value string, keys ...string) *Ctx[T] {
	c.gormScopes = append(c.gormScopes, func(tx *gorm.DB) *gorm.DB {
		return model.QueryToFussyKeysSearch(c.ctx, tx, value, keys...)
	})
	return c
}

func (c *Ctx[T]) SetEqual(keys ...string) *Ctx[T] {
	c.gormScopes = append(c.gormScopes, func(tx *gorm.DB) *gorm.DB {
		return model.QueryToEqualSearch(c.ctx, tx, keys...)
	})
	return c
}

func (c *Ctx[T]) SetIn(keys ...string) *Ctx[T] {
	c.gormScopes = append(c.gormScopes, func(tx *gorm.DB) *gorm.DB {
		return model.QueryToInSearch(c.ctx, tx, keys...)
	})
	return c
}

func (c *Ctx[T]) SetOrFussy(keys ...string) *Ctx[T] {
	c.gormScopes = append(c.gormScopes, func(tx *gorm.DB) *gorm.DB {
		return model.QueryToOrFussySearch(c.ctx, tx, keys...)
	})
	return c
}

func (c *Ctx[T]) SetOrEqual(keys ...string) *Ctx[T] {
	c.gormScopes = append(c.gormScopes, func(tx *gorm.DB) *gorm.DB {
		return model.QueryToOrEqualSearch(c.ctx, tx, keys...)
	})
	return c
}

func (c *Ctx[T]) SetOrIn(keys ...string) *Ctx[T] {
	c.gormScopes = append(c.gormScopes, func(tx *gorm.DB) *gorm.DB {
		return model.QueryToOrInSearch(c.ctx, tx, keys...)
	})
	return c
}

func (c *Ctx[T]) result() (*gorm.DB, bool) {
	for _, v := range c.preloads {
		t := v
		c.GormScope(func(tx *gorm.DB) *gorm.DB {
			tx = tx.Preload(t)
			return tx
		})
	}

	c.beforeExecuteHook()

	var dbModel T
	result := model.UseDB()

	if c.ctx.Query("trash") == "true" {
		stmt := &gorm.Statement{DB: model.UseDB()}
		err := stmt.Parse(&dbModel)
		if err != nil {
			logger.Error(err)
			return nil, false
		}
		result = result.Unscoped().Where(stmt.Schema.Table + ".deleted_at IS NOT NULL")
	}

	result = result.Model(&dbModel)

	if len(c.gormScopes) > 0 {
		result = result.Scopes(c.gormScopes...)
	}

	return result, true
}

func (c *Ctx[T]) ListAllData() ([]*T, bool) {
	result, ok := c.result()
	if !ok {
		return nil, false
	}

	result = result.Scopes(model.SortOrder(c.ctx))
	models := make([]*T, 0)
	result.Find(&models)
	return models, true
}

func (c *Ctx[T]) PagingListData() (*model.DataList, bool) {
	result, ok := c.result()
	if !ok {
		return nil, false
	}

	result = result.Scopes(model.OrderAndPaginate(c.ctx))
	data := &model.DataList{}
	if c.scan == nil {
		models := make([]*T, 0)
		result.Find(&models)

		if c.transformer != nil {
			transformed := make([]any, 0)
			for k := range models {
				transformed = append(transformed, c.transformer(models[k]))
			}
			data.Data = transformed
		} else {
			data.Data = models
		}
	} else {
		data.Data = c.scan(result)
	}

	page := cast.ToInt(c.ctx.Query("page"))
	if page == 0 {
		page = 1
	}

	pageSize := settings.AppSettings.PageSize
	if reqPageSize := c.ctx.Query("page_size"); reqPageSize != "" {
		pageSize = cast.ToInt(reqPageSize)
	}

	var totalRecords int64
	result.Session(&gorm.Session{}).Count(&totalRecords)

	data.Pagination = model.Pagination{
		Total:       totalRecords,
		PerPage:     pageSize,
		CurrentPage: page,
		TotalPages:  model.TotalPage(totalRecords, pageSize),
	}
	return data, true
}

func (c *Ctx[T]) PagingList() {
	data, ok := c.PagingListData()
	if ok {
		c.ctx.JSON(http.StatusOK, data)
	}
}
