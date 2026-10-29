package migrate

import (
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCollapseBanIPRows(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.BanIP{}))

	now := time.Now().Unix()
	rows := []model.BanIP{
		// A client stuck in the old loop: one ended window plus many
		// single-attempt rows.
		{IP: "198.51.100.1", Attempts: 10, ExpiredAt: now - 600},
		{IP: "198.51.100.1", Attempts: 1, ExpiredAt: now + 100},
		{IP: "198.51.100.1", Attempts: 3, ExpiredAt: now + 200},
		{IP: "198.51.100.1", Attempts: 1, ExpiredAt: now + 300},
		// A client with a single active row is left alone.
		{IP: "198.51.100.2", Attempts: 10, ExpiredAt: now + 500},
		// A client with only ended windows disappears.
		{IP: "198.51.100.3", Attempts: 10, ExpiredAt: now - 1},
		{IP: "198.51.100.3", Attempts: 2, ExpiredAt: now - 2},
	}
	require.NoError(t, database.Create(&rows).Error)

	var migration *gormigrate.Migration
	for _, candidate := range Migrations {
		if candidate.ID == "20261029000001" {
			migration = candidate
			break
		}
	}
	require.NotNil(t, migration, "ban_ips collapse migration is not registered")
	require.NoError(t, migration.Migrate(database))
	// Running it again must be a no-op.
	require.NoError(t, migration.Migrate(database))

	var got []model.BanIP
	require.NoError(t, database.Order("ip").Find(&got).Error)
	assert.Equal(t, []model.BanIP{
		{IP: "198.51.100.1", Attempts: 3, ExpiredAt: now + 300},
		{IP: "198.51.100.2", Attempts: 10, ExpiredAt: now + 500},
	}, got)
}

func TestCollapseBanIPRowsWithoutTable(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, CollapseBanIPRows.Migrate(database))
}
