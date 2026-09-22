package migrate

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAddStorageConfigToAutoBackups(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// A table from before plugin storage backends existed, with one S3 task.
	require.NoError(t, database.Exec(`CREATE TABLE auto_backups (
		id INTEGER PRIMARY KEY, created_at datetime, updated_at datetime, deleted_at datetime,
		name text, backup_type text, storage_type text, backup_path text, storage_path text,
		cron_expression text, enabled numeric, last_backup_time datetime, last_backup_status text,
		last_backup_error text, s3_endpoint text, s3_access_key_id text, s3_secret_access_key text,
		s3_bucket text, s3_region text)`).Error)
	require.NoError(t, database.Exec(`INSERT INTO auto_backups (id, name, backup_type, storage_type, storage_path)
		VALUES (1, 'daily', 'nginx_and_nginx_ui', 's3', 'backups')`).Error)

	require.NoError(t, AddStorageConfigToAutoBackups.Migrate(database))
	assert.True(t, database.Migrator().HasColumn(&model.AutoBackup{}, "StorageConfig"))
	assert.True(t, database.Migrator().HasColumn(&model.AutoBackup{}, "RetentionCount"))

	// The existing row keeps no plugin values and no retention limit.
	var row struct {
		StorageType    string
		StorageConfig  *string
		RetentionCount *int
	}
	require.NoError(t, database.Raw("SELECT storage_type, storage_config, retention_count FROM auto_backups WHERE id = 1").Scan(&row).Error)
	assert.Equal(t, "s3", row.StorageType)
	assert.Nil(t, row.StorageConfig)
	assert.Nil(t, row.RetentionCount)

	// Running it again is a no-op.
	require.NoError(t, AddStorageConfigToAutoBackups.Migrate(database))
}
