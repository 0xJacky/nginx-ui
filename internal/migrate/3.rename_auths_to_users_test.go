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

type beta24Auth struct {
	ID        uint64 `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Name      string
	Password  string
}

func (beta24Auth) TableName() string {
	return "auths"
}

func TestBeta24UserIsEnabledAfterRegisteredMigrationSequence(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&beta24Auth{}))
	require.NoError(t, database.Create(&beta24Auth{
		ID:       1,
		Name:     "admin",
		Password: "legacy-hash",
	}).Error)

	runRenameAuthsMigration(t, database, BeforeAutoMigrate)
	require.NoError(t, database.AutoMigrate(&model.User{}))
	runRenameAuthsMigration(t, database, Migrations)
	var migrated struct {
		Name     string
		Password string
		Status   bool
	}
	require.NoError(t, database.Table("users").Where("id = ?", 1).Take(&migrated).Error)
	assert.Equal(t, "admin", migrated.Name)
	assert.Equal(t, "legacy-hash", migrated.Password)
	assert.True(t, migrated.Status)
}

func runRenameAuthsMigration(
	t *testing.T,
	database *gorm.DB,
	migrations []*gormigrate.Migration,
) {
	t.Helper()
	for _, migration := range migrations {
		if migration.ID == RenameAuthsToUsers.ID {
			require.NoError(t, migration.Migrate(database))
		}
	}
}
