package user

import (
	"fmt"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBanIPKeepsDifferentClientBucketsSeparate(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
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
