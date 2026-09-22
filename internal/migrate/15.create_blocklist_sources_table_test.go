package migrate

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateBlocklistSourcesTable(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, CreateBlocklistSourcesTable.Migrate(database))
	assert.True(t, database.Migrator().HasTable(&model.BlocklistSource{}))
	assert.True(t, database.Migrator().HasColumn(&model.BlocklistSource{}, "next_run_at"))

	// A stored source keeps its values through the migration running again.
	require.NoError(t, database.Exec(`INSERT INTO blocklist_sources (id, name, kind, refresh_seconds, enabled, entry_count)
		VALUES (1, 'feed', 'plugin:threatfeed', 900, 1, 12)`).Error)
	require.NoError(t, CreateBlocklistSourcesTable.Migrate(database))

	var kind string
	require.NoError(t, database.Raw("SELECT kind FROM blocklist_sources WHERE id = 1").Scan(&kind).Error)
	assert.Equal(t, "plugin:threatfeed", kind)
}
