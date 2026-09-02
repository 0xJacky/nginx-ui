package migrate

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var DropLegacyRenamedTableIndexes = &gormigrate.Migration{
	ID: "20260730000001",
	Migrate: func(tx *gorm.DB) error {
		indexes := []struct {
			table   string
			legacy  string
			current string
		}{
			{table: "namespaces", legacy: "idx_env_groups_deleted_at", current: "idx_namespaces_deleted_at"},
			{table: "nodes", legacy: "idx_environments_deleted_at", current: "idx_nodes_deleted_at"},
			{table: "users", legacy: "idx_auths_deleted_at", current: "idx_users_deleted_at"},
		}

		for _, index := range indexes {
			if !tx.Migrator().HasTable(index.table) ||
				!tx.Migrator().HasIndex(index.table, index.legacy) ||
				!tx.Migrator().HasIndex(index.table, index.current) {
				continue
			}
			if err := tx.Migrator().DropIndex(index.table, index.legacy); err != nil {
				return err
			}
		}
		return nil
	},
}
