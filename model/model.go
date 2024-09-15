package model

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
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
		User{},
		AuthToken{},
		Cert{},
		ChatGPTLog{},
		Site{},
		Stream{},
		DnsCredential{},
		Environment{},
		Notification{},
		AcmeUser{},
		BanIP{},
		Config{},
		Passkey{},
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

func TotalPage(total int64, pageSize int) int64 {
	n := total / int64(pageSize)
	if total%int64(pageSize) > 0 {
		n++
	}
	return n
}

type Method interface {
	// FirstByID Where("id=@id")
	FirstByID(id int) (*gen.T, error)
	// DeleteByID update @@table set deleted_at=strftime('%Y-%m-%d %H:%M:%S','now') where id=@id
	DeleteByID(id int) error
}
