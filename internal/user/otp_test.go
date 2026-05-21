package user

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupOTPTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	return db
}

func TestVerifyOTPLegacyRecoveryCodeCanOnlyBeUsedOnce(t *testing.T) {
	db := setupOTPTestDB(t)
	secret := []byte("encrypted-otp-secret")
	legacyDigest := sha1.Sum(secret)
	legacyCode := hex.EncodeToString(legacyDigest[:])

	testUser := &model.User{
		Name:      "legacy-user",
		Status:    true,
		OTPSecret: secret,
	}
	require.NoError(t, db.Create(testUser).Error)

	result, err := VerifyOTP(testUser, "", legacyCode)
	require.NoError(t, err)
	assert.True(t, result.UsedLegacyRecoveryCode)

	var storedUser model.User
	require.NoError(t, db.First(&storedUser, testUser.ID).Error)
	require.NotNil(t, storedUser.RecoveryCodes.LegacyRecoveryCodeUsedAt)

	result, err = VerifyOTP(testUser, "", legacyCode)
	require.ErrorIs(t, err, ErrRecoveryCode)
	assert.False(t, result.UsedLegacyRecoveryCode)
}
