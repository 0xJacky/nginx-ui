package migrate

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateCertDeployTables(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, CreateCertDeployTables.Migrate(database))
	assert.True(t, database.Migrator().HasTable(&model.CertDeployTarget{}))
	assert.True(t, database.Migrator().HasTable(&model.CertDeployment{}))
	assert.True(t, database.Migrator().HasIndex(&model.CertDeployment{}, "idx_cert_deployments_target_cert"))

	// A stored target keeps its values through the migration running again.
	require.NoError(t, database.Exec(`INSERT INTO cert_deploy_targets (id, name, kind, cert_id, enabled)
		VALUES (1, 'cdn', 'plugin:mycdn', 0, 1)`).Error)
	require.NoError(t, CreateCertDeployTables.Migrate(database))

	var kind string
	require.NoError(t, database.Raw("SELECT kind FROM cert_deploy_targets WHERE id = 1").Scan(&kind).Error)
	assert.Equal(t, "plugin:mycdn", kind)
}
