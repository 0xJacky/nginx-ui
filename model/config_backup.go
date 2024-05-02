package model

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"os"
	"path/filepath"
)

type ConfigBackup struct {
	Model

	Name     string `json:"name"`
	FilePath string `json:"filepath"`
	Content  string `json:"content" gorm:"type:text"`
}

type ConfigBackupListItem struct {
	Model

	Name     string `json:"name"`
	FilePath string `json:"filepath"`
}

func GetBackupList(path string) (configs []ConfigBackupListItem) {
	db.Model(&ConfigBackup{}).
		Where(&ConfigBackup{FilePath: path}).
		Find(&configs)

	return
}

func GetBackup(id int) (config ConfigBackup) {
	db.First(&config, id)

	return
}

func CreateBackup(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		logger.Error(err)
	}

	config := ConfigBackup{Name: filepath.Base(path), FilePath: path, Content: string(content)}
	result := db.Create(&config)
	if result.Error != nil {
		logger.Error(result.Error)
	}
}
