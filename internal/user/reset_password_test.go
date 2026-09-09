package user

import (
	"fmt"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUpdateInitUserPasswordReenablesDisabledUser(t *testing.T) {
	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.User{}))
	require.NoError(t, database.Create(&model.User{
		Model:    model.Model{ID: 1},
		Name:     "admin",
		Password: "old-hash",
		Status:   true,
	}).Error)
	require.NoError(t, database.Model(&model.User{}).
		Where("id = ?", 1).Update("status", false).Error)
	var disabled bool
	require.NoError(t, database.Raw("SELECT status FROM users WHERE id = ?", 1).
		Scan(&disabled).Error)
	assert.False(t, disabled)

	model.Use(database)
	query.Use(database)
	query.SetDefault(database)

	require.NoError(t, updateInitUserPassword("new-hash"))

	var updated model.User
	require.NoError(t, database.First(&updated, 1).Error)
	assert.Equal(t, "new-hash", updated.Password)
	assert.True(t, updated.Status)
}
