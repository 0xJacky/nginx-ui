package migrate

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// CreateCertDeployTables creates the tables of the certificate deploy targets
// and their outcomes. AutoMigrate creates them as well on a fresh start; the
// migration keeps the step explicit and idempotent.
var CreateCertDeployTables = &gormigrate.Migration{
	ID: "20260923000003",
	Migrate: func(tx *gorm.DB) error {
		for _, table := range []any{&model.CertDeployTarget{}, &model.CertDeployment{}} {
			if tx.Migrator().HasTable(table) {
				continue
			}
			if err := tx.Migrator().CreateTable(table); err != nil {
				return err
			}
		}
		return nil
	},
}
