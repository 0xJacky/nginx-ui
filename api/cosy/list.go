package cosy

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gorm.io/gorm"
	"net/http"
)

func GetPagingParams(c *gin.Context) (page, offset, pageSize int) {
	page = cast.ToInt(c.Query("page"))
	if page == 0 {
		page = 1
	}
	pageSize = settings.ServerSettings.PageSize
	reqPageSize := c.Query("page_size")
	if reqPageSize != "" {
		pageSize = cast.ToInt(reqPageSize)
	}
	offset = (page - 1) * pageSize
	return
}

func (c *Ctx[T]) combineStdSelectorRequest() {
	StdSelectorInitID := c.ctx.QueryArray("id[]")

	if len(StdSelectorInitID) > 0 {
		c.GormScope(func(tx *gorm.DB) *gorm.DB {
			return tx.Where(c.itemKey+" IN ?", StdSelectorInitID)
		})
	}
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

	if cast.ToBool(c.ctx.Query("trash")) {
		stmt := &gorm.Statement{DB: model.UseDB()}
		err := stmt.Parse(&dbModel)
		if err != nil {
			logger.Error(err)
			return nil, false
		}
		result = result.Unscoped().Where(stmt.Schema.Table + ".deleted_at IS NOT NULL")
	}

	result = result.Model(&dbModel)
	if c.table != "" {
		result = result.Table(c.table, c.tableArgs...)
	}

	c.combineStdSelectorRequest()

	if len(c.gormScopes) > 0 {
		result = result.Scopes(c.gormScopes...)
	}

	return result, true
}

func (c *Ctx[T]) ListAllData() (data any, ok bool) {
	result, ok := c.result()
	if !ok {
		return nil, false
	}

	result = result.Scopes(c.SortOrder())
	if c.scan == nil {
		models := make([]*T, 0)
		result.Find(&models)

		if c.transformer != nil {
			transformed := make([]any, 0)
			for k := range models {
				transformed = append(transformed, c.transformer(models[k]))
			}
			data = transformed
		} else {
			data = models
		}
	} else {
		data = c.scan(result)
	}
	return data, true
}

func (c *Ctx[T]) PagingListData() (*model.DataList, bool) {
    result, ok := c.result()
    if !ok {
        return nil, false
    }

    scopesResult := result.Scopes(c.OrderAndPaginate())
    data := &model.DataList{}
    if c.scan == nil {
        models := make([]*T, 0)
        scopesResult.Find(&models)

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
        data.Data = c.scan(scopesResult)
    }

    var totalRecords int64
    delete(result.Statement.Clauses, "ORDER BY")
    delete(result.Statement.Clauses, "LIMIT")
    result.Count(&totalRecords)

    page := cast.ToInt(c.ctx.Query("page"))
    if page == 0 {
        page = 1
    }

    pageSize := settings.ServerSettings.PageSize
    if reqPageSize := c.ctx.Query("page_size"); reqPageSize != "" {
        pageSize = cast.ToInt(reqPageSize)
    }

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

// EmptyPagingList return empty list
func (c *Ctx[T]) EmptyPagingList() {
	pageSize := settings.ServerSettings.PageSize
	if reqPageSize := c.ctx.Query("page_size"); reqPageSize != "" {
		pageSize = cast.ToInt(reqPageSize)
	}

	data := &model.DataList{Data: make([]any, 0)}
	data.Pagination.PerPage = pageSize
	c.ctx.JSON(http.StatusOK, data)
}
