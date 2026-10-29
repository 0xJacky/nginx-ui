package migrate

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// CreateBlocklistSourcesTable creates the table of the blocklist sources.
// AutoMigrate creates it as well on a fresh start; the migration keeps the
// step explicit and idempotent.
var CreateBlocklistSourcesTable = &gormigrate.Migration{
	ID: "20260923000004",
	Migrate: func(tx *gorm.DB) error {
		if tx.Migrator().HasTable(&model.BlocklistSource{}) {
			return nil
		}
		return tx.Migrator().CreateTable(&model.BlocklistSource{})
	},
}
