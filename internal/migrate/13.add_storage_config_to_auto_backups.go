package migrate

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddStorageConfigToAutoBackups adds the plugin storage backend columns of
// the automatic backup. Existing rows keep NULL, which means no values and no
// retention limit; they use the built-in storage anyway.
var AddStorageConfigToAutoBackups = &gormigrate.Migration{
	ID: "20260923000002",
	Migrate: func(tx *gorm.DB) error {
		for _, column := range []string{"StorageConfig", "RetentionCount"} {
			if tx.Migrator().HasColumn(&model.AutoBackup{}, column) {
				continue
			}
			if err := tx.Migrator().AddColumn(&model.AutoBackup{}, column); err != nil {
				return err
			}
		}
		return nil
	},
}
