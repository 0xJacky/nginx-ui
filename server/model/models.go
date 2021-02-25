package model

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
)

var db *gorm.DB

type Model struct {
	gorm.Model
}

func Init() {
	var err error
	db, err = gorm.Open(sqlite.Open("database.db"), &gorm.Config{})
	log.Println("database.db")

	if err != nil {
		log.Println(err)
	}

	// Migrate the schema
	db.AutoMigrate(&ConfigBackup{})
}
