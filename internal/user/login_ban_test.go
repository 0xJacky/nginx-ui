package user

import (
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBanIPKeepsDifferentClientBucketsSeparate(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.BanIP{}))
	query.SetDefault(database)

	originalBanThresholdMinutes := settings.AuthSettings.BanThresholdMinutes
	t.Cleanup(func() {
		settings.AuthSettings.BanThresholdMinutes = originalBanThresholdMinutes
	})
	settings.AuthSettings.BanThresholdMinutes = 10

	BanIP("198.51.100.10")
	BanIP("203.0.113.20")
	BanIP("198.51.100.10")

	var firstClient model.BanIP
	require.NoError(t, database.Where("ip = ?", "198.51.100.10").First(&firstClient).Error)
	require.Equal(t, 2, firstClient.Attempts)

	var secondClient model.BanIP
	require.NoError(t, database.Where("ip = ?", "203.0.113.20").First(&secondClient).Error)
	require.Equal(t, 1, secondClient.Attempts)
}

// Databases written before the re-arm fix can hold an ended window next to
// several active rows for one client. BanIP must still ban that client after
// MaxAttempts failures and drop the ended row.
func TestBanIPToleratesLegacyDuplicateRows(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.BanIP{}))
	query.SetDefault(database)

	const ip = "198.51.100.10"
	now := time.Now().Unix()
	require.NoError(t, database.Create([]model.BanIP{
		{IP: ip, Attempts: settings.AuthSettings.MaxAttempts, ExpiredAt: now - 60},
		{IP: ip, Attempts: 1, ExpiredAt: now + 300},
		{IP: ip, Attempts: 1, ExpiredAt: now + 400},
	}).Error)

	for range settings.AuthSettings.MaxAttempts - 1 {
		BanIP(ip)
	}

	var rows []model.BanIP
	require.NoError(t, database.Where("ip = ?", ip).Find(&rows).Error)
	require.Len(t, rows, 2, "the ended window must be dropped")
	for _, row := range rows {
		require.GreaterOrEqual(t, row.ExpiredAt, now)
		require.Equal(t, settings.AuthSettings.MaxAttempts, row.Attempts)
	}
}
