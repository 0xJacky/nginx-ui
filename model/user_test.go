package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupUserPasskeyTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &Passkey{}))
	previousDB := UseDB()
	Use(db)
	t.Cleanup(func() {
		Use(previousDB)
	})

	return db
}

func TestUserAfterFindLoadsPasskeyStatus(t *testing.T) {
	db := setupUserPasskeyTestDB(t)

	withoutPasskey := &User{Name: "without-passkey", Status: true}
	require.NoError(t, db.Create(withoutPasskey).Error)

	var loaded User
	require.NoError(t, db.First(&loaded, withoutPasskey.ID).Error)
	assert.False(t, loaded.EnabledTwoFA)

	enabled, err := loaded.EnabledPasskey()
	require.NoError(t, err)
	assert.False(t, enabled)

	require.NoError(t, db.Create(&Passkey{UserID: withoutPasskey.ID, Name: "security-key"}).Error)
	require.NoError(t, db.First(&loaded, withoutPasskey.ID).Error)
	assert.True(t, loaded.EnabledTwoFA)
}

func TestUserAfterFindPropagatesPasskeyLookupError(t *testing.T) {
	db := setupUserPasskeyTestDB(t)

	user := &User{Name: "passkey-user", Status: true}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(&Passkey{UserID: user.ID, Name: "security-key"}).Error)
	require.NoError(t, db.Migrator().DropTable(&Passkey{}))

	enabled, err := user.EnabledPasskey()
	assert.False(t, enabled)
	require.Error(t, err)
	assert.ErrorContains(t, err, "check whether user")

	var loaded User
	err = db.First(&loaded, user.ID).Error
	require.Error(t, err)
	assert.ErrorContains(t, err, "check whether user")
}
