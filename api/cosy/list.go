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
	var StdSelectorInitParams struct {
		ID []int `json:"id"`
	}

	if err := c.ctx.ShouldBindJSON(&StdSelectorInitParams); err != nil {
		logger.Error(err)
		return
	}

	c.GormScope(func(tx *gorm.DB) *gorm.DB {
		return tx.Where(c.itemKey+" IN ?", StdSelectorInitParams.ID)
	})
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

	c.combineStdSelectorRequest()

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

	result = result.Scopes(c.SortOrder())
	models := make([]*T, 0)
	result.Find(&models)
	return models, true
}

func (c *Ctx[T]) PagingListData() (*model.DataList, bool) {
	result, ok := c.result()
	if !ok {
		return nil, false
	}

	result = result.Scopes(c.OrderAndPaginate())
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

	pageSize := settings.ServerSettings.PageSize
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
