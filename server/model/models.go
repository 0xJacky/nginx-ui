package model

import (
	"github.com/0xJacky/Nginx-UI/server/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"path"
	"time"
)

var db *gorm.DB

type Model struct {
	ID        uint       `gorm:"primary_key" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at"`
}

func Init() {
	dbPath := path.Join(settings.DataDir, "database.db")
	var err error
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger:      logger.Default.LogMode(logger.Info),
		PrepareStmt: true,
	})
	if err != nil {
		log.Println(err)
	}
	// Migrate the schema
	AutoMigrate(&ConfigBackup{})
	AutoMigrate(&Auth{})
	AutoMigrate(&AuthToken{})
	AutoMigrate(&Cert{})
}

func AutoMigrate(model interface{}) {
	err := db.AutoMigrate(model)
	if err != nil {
		log.Fatal(err)
	}
}
