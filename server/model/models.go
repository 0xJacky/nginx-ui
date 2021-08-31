package model

import (
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
    "log"
    "time"
)

var db *gorm.DB

type Model struct {
	ID        uint       `gorm:"primarykey" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at"`
}

func Init(dbPath string) {
	var err error

	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
        Logger:      logger.Default.LogMode(logger.Info),
        PrepareStmt: true,
    })
	log.Println("database.db")

	if err != nil {
		log.Println(err)
	}

	// Migrate the schema
	AutoMigrate(&ConfigBackup{})
	AutoMigrate(&Auth{})
	AutoMigrate(&AuthToken{})
}

func AutoMigrate(model interface{}) {
	err := db.AutoMigrate(model)
	if err != nil {
		log.Fatal(err)
	}
}
