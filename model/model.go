package model

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gorm.io/driver/sqlite"
	"gorm.io/gen"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"path"
	"time"
)

var db *gorm.DB

type Model struct {
	ID        int             `gorm:"primary_key" json:"id"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	DeletedAt *gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

func GenerateAllModel() []any {
	return []any{
		ConfigBackup{},
		Auth{},
		AuthToken{},
		Cert{},
		ChatGPTLog{},
		Site{},
		DnsCredential{},
		Environment{},
	}
}

func logMode() gormlogger.Interface {
	switch settings.ServerSettings.RunMode {
	case gin.ReleaseMode:
		return gormlogger.Default.LogMode(gormlogger.Warn)
	default:
		fallthrough
	case gin.DebugMode:
		return gormlogger.Default.LogMode(gormlogger.Info)
	}
}

func Init() *gorm.DB {
	dbPath := path.Join(path.Dir(settings.ConfPath), fmt.Sprintf("%s.db", settings.ServerSettings.Database))

	var err error
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger:                                   logMode(),
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: true,
	})

	if err != nil {
		logger.Fatal(err.Error())
	}

	// Migrate the schema
	err = db.AutoMigrate(GenerateAllModel()...)
	if err != nil {
		logger.Fatal(err.Error())
	}

	return db
}

func orderAndPaginate(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		sort := c.DefaultQuery("sort", "desc")
		order := c.DefaultQuery("order_by", "id") +
			" " + sort

		page := cast.ToInt(c.Query("page"))
		if page == 0 {
			page = 1
		}
		pageSize := settings.ServerSettings.PageSize
		reqPageSize := c.Query("page_size")
		if reqPageSize != "" {
			pageSize = cast.ToInt(reqPageSize)
		}
		offset := (page - 1) * pageSize

		return db.Order(order).Offset(offset).Limit(pageSize)
	}
}

func totalPage(total int64, pageSize int) int64 {
	n := total / int64(pageSize)
	if total%int64(pageSize) > 0 {
		n++
	}
	return n
}

type Pagination struct {
	Total       int64 `json:"total"`
	PerPage     int   `json:"per_page"`
	CurrentPage int   `json:"current_page"`
	TotalPages  int64 `json:"total_pages"`
}

type DataList struct {
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination,omitempty"`
}

func GetListWithPagination(models interface{},
	c *gin.Context, totalRecords int64) (result DataList) {

	page := cast.ToInt(c.Query("page"))
	if page == 0 {
		page = 1
	}

	result = DataList{}

	result.Data = models

	pageSize := settings.ServerSettings.PageSize
	reqPageSize := c.Query("page_size")
	if reqPageSize != "" {
		pageSize = cast.ToInt(reqPageSize)
	}

	result.Pagination = Pagination{
		Total:       totalRecords,
		PerPage:     pageSize,
		CurrentPage: page,
		TotalPages:  totalPage(totalRecords, pageSize),
	}

	return
}

type Method interface {
	// FirstByID Where("id=@id")
	FirstByID(id int) (*gen.T, error)
	// DeleteByID update @@table set deleted_at=strftime('%Y-%m-%d %H:%M:%S','now') where id=@id
	DeleteByID(id int) error
}
