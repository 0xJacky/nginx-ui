package user

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTokenAuthTest(t *testing.T) (*gorm.DB, *model.User, *model.AuthToken) {
	t.Helper()

	cache.InitInMemoryCache()
	cSettings.AppSettings.JwtSecret = "test-secret"

	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}))

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

	authToken := &model.AuthToken{}
	require.NoError(t, db.Where("token = ?", token.Token).First(authToken).Error)

	return db, testUser, authToken
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

func TestDeleteUserTokensClearsStandaloneShortTokenCache(t *testing.T) {
	db, testUser, _ := setupTokenAuthTest(t)

	shortToken := "legacyshorttoken"
	legacy := &model.AuthToken{UserID: testUser.ID, ShortToken: shortToken, ExpiredAt: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, db.Create(legacy).Error)
	CacheToken(legacy)

	_, found := GetCachedShortTokenData(shortToken)
	require.True(t, found)

	require.NoError(t, db.Model(&model.User{}).Where("id = ?", testUser.ID).Update("status", false).Error)
	DeleteUserTokens(testUser.ID)

	var tokenCount int64
	require.NoError(t, db.Model(&model.AuthToken{}).Where("short_token = ?", shortToken).Count(&tokenCount).Error)
	assert.Zero(t, tokenCount)

	_, found = GetCachedShortTokenData(shortToken)
	assert.False(t, found)

	require.NoError(t, db.Model(&model.User{}).Where("id = ?", testUser.ID).Update("status", true).Error)

	resurrectedUser, ok := GetTokenUserByShortToken(shortToken)
	assert.False(t, ok)
	assert.Nil(t, resurrectedUser)
}

func TestLegacyStandaloneShortTokenFailsClosedWithoutDeletingOtherSessions(t *testing.T) {
	db, testUser, parent := setupTokenAuthTest(t)
	legacy := &model.AuthToken{UserID: testUser.ID, ShortToken: "legacyshorttoken", ExpiredAt: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, db.Create(legacy).Error)
	CacheToken(legacy)

	_, ok := GetTokenUserByShortToken(legacy.ShortToken)
	require.False(t, ok)
	var count int64
	require.NoError(t, db.Model(&model.AuthToken{}).Where("short_token = ?", legacy.ShortToken).Count(&count).Error)
	require.Zero(t, count)
	_, cached := GetCachedShortTokenData(legacy.ShortToken)
	require.False(t, cached)
	_, ok = GetTokenUser(parent.Token)
	require.True(t, ok)

	cache.InitInMemoryCache()
	InitTokenCache(context.Background())
	_, ok = GetTokenUserByShortToken(legacy.ShortToken)
	require.False(t, ok)
}

func TestSessionShortTokenFailsClosedAfterParentRevocationAndCacheReload(t *testing.T) {
	db, testUser, parent := setupTokenAuthTest(t)
	shortToken, err := GenerateShortTokenForSession(testUser.ID, parent.Token)
	require.NoError(t, err)

	cache.InitInMemoryCache()
	InitTokenCache(context.Background())
	loadedUser, ok := GetTokenUserByShortToken(shortToken)
	require.True(t, ok)
	require.Equal(t, testUser.ID, loadedUser.ID)

	require.NoError(t, RevokeSessionToken(parent.Token))
	cache.InitInMemoryCache()
	InitTokenCache(context.Background())
	_, ok = GetTokenUserByShortToken(shortToken)
	require.False(t, ok)
	var count int64
	require.NoError(t, db.Model(&model.AuthToken{}).Where("short_token = ?", shortToken).Count(&count).Error)
	require.Zero(t, count)
}

func TestSessionLookupErrorDoesNotRevokeValidLogin(t *testing.T) {
	db, testUser, parent := setupTokenAuthTest(t)
	shortToken, err := GenerateShortTokenForSession(testUser.ID, parent.Token)
	require.NoError(t, err)

	callbackName := "test:fail_session_lookup"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*int64); ok {
			tx.AddError(errors.New("temporary session lookup failure"))
		}
	}))
	_, ok := GetTokenUserByShortToken(shortToken)
	require.False(t, ok)
	require.NoError(t, db.Callback().Query().Remove(callbackName))

	_, ok = GetTokenUser(parent.Token)
	require.True(t, ok)
	_, ok = GetTokenUserByShortToken(shortToken)
	require.True(t, ok)
}

func TestAuthTokenSessionHashMigrationPreservesLegacyRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s-migration?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE auth_tokens (user_id integer, token text, short_token text, expired_at integer)").Error)
	require.NoError(t, db.Exec("INSERT INTO auth_tokens (user_id, token, short_token, expired_at) VALUES (?, ?, ?, ?)", 7, "legacy-jwt", "legacy-short", int64(123)).Error)

	require.NoError(t, db.AutoMigrate(&model.AuthToken{}))
	require.True(t, db.Migrator().HasColumn(&model.AuthToken{}, "session_hash"))
	require.True(t, db.Migrator().HasIndex(&model.AuthToken{}, "idx_auth_tokens_token"))
	require.True(t, db.Migrator().HasIndex(&model.AuthToken{}, "idx_auth_tokens_short_token"))
	require.True(t, db.Migrator().HasIndex(&model.AuthToken{}, "idx_auth_tokens_session_hash"))
	var legacy model.AuthToken
	require.NoError(t, db.Where("token = ?", "legacy-jwt").Take(&legacy).Error)
	require.Equal(t, uint64(7), legacy.UserID)
	require.Equal(t, "legacy-short", legacy.ShortToken)
	require.Empty(t, legacy.SessionHash)
}

func TestIssueLoginTokenRequiresPasskeyProofForPasskeyOnlyUser(t *testing.T) {
	db, testUser, _ := setupTokenAuthTest(t)
	DeleteUserTokens(testUser.ID)
	require.NoError(t, db.Create(&model.Passkey{
		UserID: testUser.ID,
		Name:   "security-key",
		RawID:  "credential-id",
	}).Error)

	token, err := IssueLoginToken(testUser, LoginProofPassword)
	require.ErrorIs(t, err, ErrPasskeyRequired)
	assert.Nil(t, token)

	var count int64
	require.NoError(t, db.Model(&model.AuthToken{}).Where("user_id = ?", testUser.ID).Count(&count).Error)
	assert.Zero(t, count, "password verification must not persist a token before passkey proof")

	token, err = IssueLoginToken(testUser, LoginProofPasskey)
	require.NoError(t, err)
	assert.NotEmpty(t, token.Token)
}

func TestIssueLoginTokenFailsClosedWhenPasskeyLookupFails(t *testing.T) {
	db, testUser, _ := setupTokenAuthTest(t)
	DeleteUserTokens(testUser.ID)
	require.NoError(t, db.Create(&model.Passkey{
		UserID: testUser.ID,
		Name:   "security-key",
		RawID:  "credential-id",
	}).Error)
	require.NoError(t, db.Migrator().DropTable(&model.Passkey{}))

	token, err := IssueLoginToken(testUser, LoginProofPassword)
	require.Error(t, err)
	assert.Nil(t, token)
	assert.ErrorContains(t, err, "check whether user")

	var count int64
	require.NoError(t, db.Model(&model.AuthToken{}).Where("user_id = ?", testUser.ID).Count(&count).Error)
	assert.Zero(t, count, "a failed passkey lookup must not persist a token")
}

func TestPasswordLoginFailsClosedWhenPasskeyLookupFails(t *testing.T) {
	db, testUser, _ := setupTokenAuthTest(t)
	DeleteUserTokens(testUser.ID)

	password := "correct-password"
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, db.Model(testUser).Update("password", string(passwordHash)).Error)
	require.NoError(t, db.Create(&model.Passkey{
		UserID: testUser.ID,
		Name:   "security-key",
		RawID:  "credential-id",
	}).Error)
	require.NoError(t, db.Migrator().DropTable(&model.Passkey{}))

	authenticatedUser, err := Login(testUser.Name, password)
	require.Error(t, err)
	assert.Nil(t, authenticatedUser)
	assert.ErrorContains(t, err, "check whether user")

	var count int64
	require.NoError(t, db.Model(&model.AuthToken{}).Where("user_id = ?", testUser.ID).Count(&count).Error)
	assert.Zero(t, count, "password login must not persist a token when passkey state is unknown")
}

func TestIssueLoginTokenAllowsPasswordForUserWithoutPasskey(t *testing.T) {
	_, testUser, _ := setupTokenAuthTest(t)
	DeleteUserTokens(testUser.ID)

	token, err := IssueLoginToken(testUser, LoginProofPassword)
	require.NoError(t, err)
	assert.NotEmpty(t, token.Token)
}
