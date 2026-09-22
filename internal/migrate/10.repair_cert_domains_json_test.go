package migrate

import (
	"testing"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRepairCertDomainsJSON(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Exec("CREATE TABLE certs (id INTEGER PRIMARY KEY, domains text)").Error)

	rows := []struct {
		id      int
		domains any
		want    any
	}{
		{id: 1, domains: "example.internal", want: `["example.internal"]`},
		{id: 2, domains: " a.test, b.test ", want: `["a.test","b.test"]`},
		{id: 3, domains: `["valid.test"]`, want: `["valid.test"]`},
		{id: 4, domains: "[]", want: "[]"},
		{id: 5, domains: nil, want: nil},
		{id: 6, domains: "", want: ""},
	}
	for _, row := range rows {
		require.NoError(t, database.Exec("INSERT INTO certs (id, domains) VALUES (?, ?)", row.id, row.domains).Error)
	}

	var migration *gormigrate.Migration
	for _, candidate := range Migrations {
		if candidate.ID == "20260922000001" {
			migration = candidate
			break
		}
	}
	require.NotNil(t, migration, "cert domains repair migration is not registered")
	require.NoError(t, migration.Migrate(database))
	// Running it again must be a no-op.
	require.NoError(t, migration.Migrate(database))

	for _, row := range rows {
		var got *string
		require.NoError(t, database.Raw("SELECT domains FROM certs WHERE id = ?", row.id).Scan(&got).Error)
		if row.want == nil {
			assert.Nil(t, got, "row %d", row.id)
			continue
		}
		require.NotNil(t, got, "row %d", row.id)
		assert.Equal(t, row.want, *got, "row %d", row.id)
	}
}

func TestRepairCertDomainsJSONWithoutCertsTable(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, RepairCertDomainsJSON.Migrate(database))
}
