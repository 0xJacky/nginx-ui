package migrate

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAddProbeKindToSiteConfigs(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// A table from before probe kinds existed, with one configured site.
	require.NoError(t, database.Exec(`CREATE TABLE site_configs (
		id INTEGER PRIMARY KEY, created_at datetime, updated_at datetime, deleted_at datetime,
		site_key text, site_name text, host text, port integer, scheme text, display_url text,
		custom_order integer, health_check_enabled numeric, check_interval integer, timeout integer,
		user_agent text, max_redirects integer, follow_redirects numeric, check_favicon numeric,
		health_check_config text, health_check_alert text)`).Error)
	require.NoError(t, database.Exec(`INSERT INTO site_configs (id, site_key, host) VALUES (1, 'example', 'example.com:443')`).Error)

	require.NoError(t, AddProbeKindToSiteConfigs.Migrate(database))
	assert.True(t, database.Migrator().HasColumn(&model.SiteConfig{}, "ProbeKind"))
	assert.True(t, database.Migrator().HasColumn(&model.SiteConfig{}, "ProbeConfig"))

	// The existing row selects the built-in check.
	var kind *string
	require.NoError(t, database.Raw("SELECT probe_kind FROM site_configs WHERE id = 1").Scan(&kind).Error)
	assert.Nil(t, kind)

	// Running it again is a no-op.
	require.NoError(t, AddProbeKindToSiteConfigs.Migrate(database))
}
