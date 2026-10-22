package backup

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func createSQLiteBackupFixture(t *testing.T, path string) {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)").Error)
	sqlDatabase, err := database.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDatabase.Close())
}
