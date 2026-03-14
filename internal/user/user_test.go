package user

import (
	"fmt"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTokenAuthTest(t *testing.T) (*gorm.DB, *model.User, *AccessTokenPayload) {
	t.Helper()

	cache.InitInMemoryCache()
	cSettings.AppSettings.JwtSecret = "test-secret"

	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuthToken{}))

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	testUser := &model.User{
		Name:     "token-user",
		Status:   true,
		Language: "en",
	}
	require.NoError(t, db.Create(testUser).Error)

	token, err := GenerateJWT(testUser)
	require.NoError(t, err)

	return db, testUser, token
}

func TestGetTokenUserRejectsDisabledUserWithCachedUser(t *testing.T) {
	db, testUser, token := setupTokenAuthTest(t)

	loadedUser, ok := GetTokenUser(token.Token)
	require.True(t, ok)
	require.NotNil(t, loadedUser)

	_, found := GetCachedUser(testUser.ID)
	require.True(t, found)

	require.NoError(t, db.Model(&model.User{}).Where("id = ?", testUser.ID).Update("status", false).Error)

	disabledUser, ok := GetTokenUser(token.Token)
	assert.False(t, ok)
	assert.Nil(t, disabledUser)

	var tokenCount int64
	require.NoError(t, db.Model(&model.AuthToken{}).Where("user_id = ?", testUser.ID).Count(&tokenCount).Error)
	assert.Zero(t, tokenCount)

	_, found = GetCachedTokenData(token.Token)
	assert.False(t, found)
}

func TestGetTokenUserByShortTokenRejectsDisabledUserWithCachedUser(t *testing.T) {
	db, testUser, token := setupTokenAuthTest(t)

	loadedUser, ok := GetTokenUserByShortToken(token.ShortToken)
	require.True(t, ok)
	require.NotNil(t, loadedUser)

	_, found := GetCachedUser(testUser.ID)
	require.True(t, found)

	require.NoError(t, db.Model(&model.User{}).Where("id = ?", testUser.ID).Update("status", false).Error)

	disabledUser, ok := GetTokenUserByShortToken(token.ShortToken)
	assert.False(t, ok)
	assert.Nil(t, disabledUser)

	var tokenCount int64
	require.NoError(t, db.Model(&model.AuthToken{}).Where("user_id = ?", testUser.ID).Count(&tokenCount).Error)
	assert.Zero(t, tokenCount)

	_, found = GetCachedShortTokenData(token.ShortToken)
	assert.False(t, found)
}

func TestDeleteUserTokensClearsTokenAndUserCaches(t *testing.T) {
	db, testUser, token := setupTokenAuthTest(t)

	CacheUser(testUser)

	_, found := GetCachedTokenData(token.Token)
	require.True(t, found)

	_, found = GetCachedShortTokenData(token.ShortToken)
	require.True(t, found)

	_, found = GetCachedUser(testUser.ID)
	require.True(t, found)

	DeleteUserTokens(testUser.ID)

	var tokenCount int64
	require.NoError(t, db.Model(&model.AuthToken{}).Where("user_id = ?", testUser.ID).Count(&tokenCount).Error)
	assert.Zero(t, tokenCount)

	_, found = GetCachedTokenData(token.Token)
	assert.False(t, found)

	_, found = GetCachedShortTokenData(token.ShortToken)
	assert.False(t, found)

	_, found = GetCachedUser(testUser.ID)
	assert.False(t, found)
}
