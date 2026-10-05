package migrate

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddStorageConfigToAutoBackups adds the plugin storage backend column of
// the automatic backup. Existing rows keep NULL, which means no values; they
// use the built-in storage anyway. The retention count every storage shares
// comes with the model.
var AddStorageConfigToAutoBackups = &gormigrate.Migration{
	ID: "20260923000002",
	Migrate: func(tx *gorm.DB) error {
		if tx.Migrator().HasColumn(&model.AutoBackup{}, "StorageConfig") {
			return nil
		}
		return tx.Migrator().AddColumn(&model.AutoBackup{}, "StorageConfig")
	},
}
