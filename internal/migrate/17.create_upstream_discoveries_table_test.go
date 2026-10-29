package migrate

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateUpstreamDiscoveriesTable(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, CreateUpstreamDiscoveriesTable.Migrate(database))
	assert.True(t, database.Migrator().HasTable(&model.UpstreamDiscovery{}))
	assert.True(t, database.Migrator().HasColumn(&model.UpstreamDiscovery{}, "extra_directives"))

	// A stored binding keeps its values through the migration running again.
	require.NoError(t, database.Exec(`INSERT INTO upstream_discoveries (id, upstream_name, kind, service, refresh_seconds, enabled, target_count)
		VALUES (1, 'api', 'plugin:registry', 'api', 60, 1, 3)`).Error)
	require.NoError(t, CreateUpstreamDiscoveriesTable.Migrate(database))

	var name string
	require.NoError(t, database.Raw("SELECT upstream_name FROM upstream_discoveries WHERE id = 1").Scan(&name).Error)
	assert.Equal(t, "api", name)
}
