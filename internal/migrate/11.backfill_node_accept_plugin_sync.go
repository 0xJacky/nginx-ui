package migrate

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// BackfillNodeAcceptPluginSync opts every node that existed before plugin
// cluster sync into it. The column default only covers the rows the database
// backfills itself, which is not guaranteed across drivers.
var BackfillNodeAcceptPluginSync = &gormigrate.Migration{
	ID: "20260922000002",
	Migrate: func(tx *gorm.DB) error {
		if !tx.Migrator().HasTable("nodes") || !tx.Migrator().HasColumn("nodes", "accept_plugin_sync") {
			return nil
		}
		return tx.Exec("UPDATE nodes SET accept_plugin_sync = ?", true).Error
	},
}
