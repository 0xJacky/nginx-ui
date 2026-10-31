package migrate

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// CreateUpstreamDiscoveriesTable creates the table of the upstream
// discoveries. AutoMigrate creates it as well on a fresh start; the
// migration keeps the step explicit and idempotent.
var CreateUpstreamDiscoveriesTable = &gormigrate.Migration{
	ID: "20260923000005",
	Migrate: func(tx *gorm.DB) error {
		if tx.Migrator().HasTable(&model.UpstreamDiscovery{}) {
			return nil
		}
		return tx.Migrator().CreateTable(&model.UpstreamDiscovery{})
	},
}
