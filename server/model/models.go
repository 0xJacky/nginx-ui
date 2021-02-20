package model

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

type Model struct {
	gorm.Model
}

func getCurrentPath() string {
	s, err := exec.LookPath(os.Args[0])
	if err != nil {
		log.Println(err)
	}
	i := strings.LastIndex(s, "\\")
	path := string(s[0 : i+1])
	return path
}

func Setup() {
	var err error
	db, err = gorm.Open(sqlite.Open(filepath.Join(getCurrentPath(), "database.db")), &gorm.Config{})
	log.Println(filepath.Join(getCurrentPath(), "database.db"))

	if err != nil {
		log.Println(err)
	}

	// Migrate the schema
	db.AutoMigrate(&ConfigBackup{})
}
