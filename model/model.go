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
	"strings"
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

func UseDB() *gorm.DB {
	return db
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

func SortOrder(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		sort := c.DefaultQuery("order", "desc")
		order := fmt.Sprintf("`%s` %s", DefaultQuery(c, "sort_by", "id"), sort)
		return db.Order(order)
	}
}

func OrderAndPaginate(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		sort := c.DefaultQuery("order", "desc")

		sortBy := DefaultQuery(c, "sort_by", "")

		if sortBy != "" {
			order := fmt.Sprintf("`%s` %s", DefaultQuery(c, "sort_by", "id"), sort)
			db = db.Order(order)
		}

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

		return db.Offset(offset).Limit(pageSize)
	}
}

func QueryToInSearch(c *gin.Context, db *gorm.DB, keys ...string) *gorm.DB {
	for _, v := range keys {
		queryArray := c.QueryArray(v + "[]")
		if len(queryArray) == 0 {
			queryArray = c.QueryArray(v)
		}
		if len(queryArray) > 0 {
			var sb strings.Builder

			_, err := fmt.Fprintf(&sb, "`%s` IN ?", v)
			if err != nil {
				logger.Error(err)
				continue
			}

			db = db.Where(sb.String(), queryArray)
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
		if len(queryArray) > 0 {
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

func TotalPage(total int64, pageSize int) int64 {
	n := total / int64(pageSize)
	if total%int64(pageSize) > 0 {
		n++
	}
	return n
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

type Method interface {
	// FirstByID Where("id=@id")
	FirstByID(id int) (*gen.T, error)
	// DeleteByID update @@table set deleted_at=strftime('%Y-%m-%d %H:%M:%S','now') where id=@id
	DeleteByID(id int) error
}
