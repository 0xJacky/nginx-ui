package model

import (
	"fmt"
	"log"
	"path"
	"time"

	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

type Service struct {
	DB *gorm.DB
}

func NewService(db *gorm.DB) Service {
	return Service{DB: db}
}

type Model struct {
	ID        uint       `gorm:"primary_key" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at"`
}

func Init() (*gorm.DB, error) {
	dbPath := path.Join(path.Dir(settings.ConfPath), fmt.Sprintf("%s.db", settings.ServerSettings.Database))
	var err error
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger:      logger.Default.LogMode(logger.Info),
		PrepareStmt: true,
	})
	if err != nil {
		return nil, err
	}
	// Migrate the schema
	AutoMigrate(&ConfigBackup{})
	AutoMigrate(&Auth{})
	AutoMigrate(&AuthToken{})
	AutoMigrate(&Cert{})
	AutoMigrate(&Log{})
	return db, nil
}

func AutoMigrate(model interface{}) {
	err := db.AutoMigrate(model)
	if err != nil {
		log.Fatal(err)
	}
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
