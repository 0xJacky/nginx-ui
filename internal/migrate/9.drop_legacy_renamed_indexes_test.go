package migrate

import (
	"testing"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDropLegacyRenamedTableIndexes(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	indexes := []struct {
		table   string
		legacy  string
		current string
	}{
		{table: "namespaces", legacy: "idx_env_groups_deleted_at", current: "idx_namespaces_deleted_at"},
		{table: "nodes", legacy: "idx_environments_deleted_at", current: "idx_nodes_deleted_at"},
		{table: "users", legacy: "idx_auths_deleted_at", current: "idx_users_deleted_at"},
	}

	for _, item := range indexes {
		require.NoError(t, database.Exec("CREATE TABLE "+item.table+" (id INTEGER PRIMARY KEY, deleted_at datetime)").Error)
		require.NoError(t, database.Exec("CREATE INDEX "+item.legacy+" ON "+item.table+" (deleted_at)").Error)
		require.NoError(t, database.Exec("CREATE INDEX "+item.current+" ON "+item.table+" (deleted_at)").Error)
	}

	var migration *gormigrate.Migration
	for _, candidate := range Migrations {
		if candidate.ID == "20260730000001" {
			migration = candidate
			break
		}
	}
	require.NotNil(t, migration, "legacy index cleanup migration is not registered")
	require.NoError(t, migration.Migrate(database))

	for _, item := range indexes {
		assert.False(t, database.Migrator().HasIndex(item.table, item.legacy))
		assert.True(t, database.Migrator().HasIndex(item.table, item.current))
	}
}
